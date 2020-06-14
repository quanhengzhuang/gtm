package gtm

import (
	"fmt"
	"time"
)

// Transaction is the definition of GTM transaction.
// Including multiple NormalPartners, one UncertainPartner, multiple CertainPartners.
type Transaction struct {
	ID   string
	Name string

	Times     int
	RetryTime time.Time
	Timeout   time.Duration

	NormalPartners   []NormalPartner
	UncertainPartner UncertainPartner
	CertainPartners  []CertainPartner
	AsyncPartners    []CertainPartner

	storage Storage
	timer   Timer
}

type Result string

const (
	// Use for Partner or Transaction.
	Success   Result = "success"
	Fail      Result = "fail"
	Uncertain Result = "uncertain"

	// Use for Transaction only.
	doNextRetrying Result = "doNextRetrying"
	undoRetrying   Result = "undoRetrying"
)

var (
	defaultStorage Storage
	defaultTimer   Timer = &DoubleTimer{}
	defaultTimeout       = 60 * time.Second
)

func New() *Transaction {
	return &Transaction{
		storage: defaultStorage,
		timer:   defaultTimer,
	}
}

func (tx *Transaction) SetName(name string) *Transaction {
	tx.Name = name
	return tx
}

func (tx *Transaction) SetTimeout(timeout time.Duration) *Transaction {
	tx.Timeout = timeout
	return tx
}

func SetStorage(storage Storage) {
	defaultStorage = storage
}

func (tx *Transaction) AddNormalPartners(partners ...NormalPartner) *Transaction {
	tx.NormalPartners = append(tx.NormalPartners, partners...)
	return tx
}

func (tx *Transaction) AddUncertainPartner(partner UncertainPartner) *Transaction {
	tx.UncertainPartner = partner
	return tx
}

func (tx *Transaction) AddCertainPartners(partners ...CertainPartner) *Transaction {
	tx.CertainPartners = append(tx.CertainPartners, partners...)
	return tx
}

func (tx *Transaction) AddAsyncPartners(partners ...CertainPartner) *Transaction {
	tx.AsyncPartners = append(tx.AsyncPartners, partners...)
	return tx
}

// ExecuteBackground will return immediately.
func (tx *Transaction) ExecuteBackground() (err error) {
	if tx.storage == nil {
		return fmt.Errorf("storage is nil")
	}

	tx.RetryTime = tx.timer.CalcRetryTime(0, tx.Timeout)
	if _, err := tx.storage.SaveTransaction(tx); err != nil {
		return fmt.Errorf("save transaction failed: %v", err)
	}

	return nil
}

// RetryTimeoutTransactions retry to complete timeout transactions.
// Count is used to set the total number of transactions per retry.
// Returns the total number of actual retries, and retry errors.
func RetryTimeoutTransactions(count int) (transactions []*Transaction, results []Result, errs []error, err error) {
	transactions, err = defaultStorage.GetTimeoutTransactions(count)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get timeout transactions err: %v", err)
	}

	for _, tx := range transactions {
		result, err := tx.ExecuteRetry()
		errs = append(errs, err)
		results = append(results, result)
	}

	return transactions, results, errs, nil
}

// ExecuteRetry use to complete the transaction.
func (tx *Transaction) ExecuteRetry() (result Result, err error) {
	// todo write once
	tx.storage = defaultStorage
	tx.timer = defaultTimer

	retryTime := tx.timer.CalcRetryTime(tx.Times+1, tx.Timeout)
	if err := tx.storage.SetTransactionRetryTime(tx, tx.Times+1, retryTime); err != nil {
		return Uncertain, fmt.Errorf("set transaction retry time err: %v", err)
	}

	return tx.execute()
}

// Execute the transaction and return the final result.
func (tx *Transaction) Execute() (result Result, err error) {
	if tx.storage == nil {
		return Fail, fmt.Errorf("storage is nil")
	}

	tx.Times = 1
	tx.RetryTime = tx.timer.CalcRetryTime(0, tx.Timeout)
	if tx.ID, err = tx.storage.SaveTransaction(tx); err != nil {
		return Fail, fmt.Errorf("save transaction failed: %v", err)
	}

	return tx.execute()
}

func (tx *Transaction) execute() (result Result, err error) {
	result, undoOffset, err := tx.do()

	switch result {
	case Success:
		if err := tx.doNext(); err != nil {
			_ = tx.storage.SaveTransactionResult(tx, doNextRetrying)
			return Uncertain, fmt.Errorf("doNext() failed: %v", err)
		}

		if err := tx.storage.SaveTransactionResult(tx, Success); err != nil {
			return Uncertain, fmt.Errorf("save transaction result failed: %v, %v", Success, err)
		}

		return Success, nil
	case Fail:
		if err := tx.undo(undoOffset); err != nil {
			_ = tx.storage.SaveTransactionResult(tx, undoRetrying)
			return Uncertain, fmt.Errorf("undo() failed: %v", err)
		}

		if err := tx.storage.SaveTransactionResult(tx, Fail); err != nil {
			return Uncertain, fmt.Errorf("save transaction result failed: %v, %v", Fail, err)
		}

		return Fail, err
	default:
		return Uncertain, fmt.Errorf("do err: %v", err)
	}
}

// do is used to execute uncertain operations.
// Equivalent to the Prepare phase in 2PC.
func (tx *Transaction) do() (result Result, undoOffset int, err error) {
	result, undoOffset, err = tx.doNormal()
	if result != Success {
		return result, undoOffset, fmt.Errorf("doNormal failed: %v", err)
	}

	return tx.doUncertain()
}

func (tx *Transaction) doNormal() (result Result, undoOffset int, err error) {
	phase := "do-normal"

	for i, partner := range tx.NormalPartners {
		if result = tx.getPartnerResult(phase, i); result == "" {
			result, err = partner.Do()
			if err := tx.storage.SavePartnerResult(tx, phase, i, result); err != nil {
				return Uncertain, i, fmt.Errorf("save partner result failed: %v, %v, %v, %v", phase, i, result, err)
			}
		}

		switch result {
		case Success:
			// continue
		case Fail:
			return Fail, i - 1, fmt.Errorf("do's failed: %v", err)
		case Uncertain:
			return Fail, i, fmt.Errorf("do's uncertain: %v", err)
		default:
			panic("unexpect result value: " + result)
		}
	}

	return Success, 0, nil
}

func (tx *Transaction) doUncertain() (result Result, undoOffset int, err error) {
	if tx.UncertainPartner == nil {
		return Success, 0, nil
	}

	phase := "do-uncertain"

	if result = tx.getPartnerResult(phase, 0); result == "" {
		result, err = tx.UncertainPartner.Do()
		if result == Success || result == Fail {
			if err := tx.storage.SavePartnerResult(tx, phase, 0, result); err != nil {
				return Uncertain, 0, fmt.Errorf("save partner result failed: %v, %v, %v", phase, result, err)
			}
		}
	}

	switch result {
	case Success:
		return Success, 0, nil
	case Fail:
		return Fail, len(tx.NormalPartners) - 1, fmt.Errorf("partner do failed: %v", err)
	case Uncertain:
		return Uncertain, 0, fmt.Errorf("partner return err: %v, %v, %v", phase, result, err)
	default:
		panic("unexpect result value: " + result)
	}
}

// doNext is used to supplement do.
// Equivalent to the Commit phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (tx *Transaction) doNext() error {
	if err := tx.doNextNormal(); err != nil {
		return fmt.Errorf("normalPartner DoNext() failed: %v", err)
	}

	if err := tx.doNextCertain(); err != nil {
		return fmt.Errorf("certainPartner DoNext() failed: %v", err)
	}

	return nil
}

func (tx *Transaction) doNextNormal() (err error) {
	phase := "doNext-normal"

	for i, v := range tx.NormalPartners {
		if result := tx.getPartnerResult(phase, i); result == "" {
			if err = v.DoNext(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage.SavePartnerResult(tx, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

func (tx *Transaction) doNextCertain() error {
	phase := "doNext-certain"

	for i, v := range tx.CertainPartners {
		if result := tx.getPartnerResult(phase, i); result == "" {
			if err := v.DoNext(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage.SavePartnerResult(tx, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

// undo will rollback all successful do.
// Equivalent to the Rollback phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (tx *Transaction) undo(undoOffset int) error {
	return tx.undoNormal(undoOffset)
}

func (tx *Transaction) undoNormal(undoOffset int) error {
	phase := "undo-normal"

	for i := undoOffset; i >= 0; i-- {
		if result := tx.getPartnerResult(phase, i); result == "" {
			if err := tx.NormalPartners[i].Undo(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage.SavePartnerResult(tx, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

func (tx *Transaction) getPartnerResult(phase string, offset int) (result Result) {
	if tx.Times == 0 {
		return ""
	}

	var err error
	if result, err = tx.storage.GetPartnerResult(tx, phase, offset); err != nil {
		return ""
	}

	return result
}
