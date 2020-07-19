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

	Times   int
	StartAt time.Time
	RetryAt time.Time
	Timeout time.Duration

	NormalPartners   []NormalPartner
	UncertainPartner UncertainPartner
	CertainPartners  []CertainPartner
	AsyncPartners    []CertainPartner
}

type Result string

const (
	// Use for Partner or Transaction.
	Success   Result = "success"
	Fail      Result = "fail"
	Uncertain Result = "uncertain"
)

var (
	// Default storage is nil, must be set when first used.
	// SetStorage() to switch default storage.
	defaultStorage Storage = nil

	// SetTimer() to switch default timer.
	defaultTimer Timer = &DoubleTimer{}

	// SetDoer() to switch default doer.
	defaultDoer Doer = &SequenceDoer{}

	// Default timeout of a transaction is 60 seconds.
	// The transaction will be retried after the first timeout.
	// Call tx.SetTimeout() to change.
	defaultTimeout = 60 * time.Second
)

// New returns an empty GTM transaction.
func New(name string) *Transaction {
	return &Transaction{Name: name}
}

// SetStorage is used to set the default storage engine.
// The setting is effective for all transactions.
// The initial value of the storage is nil and must be set.
func SetStorage(s Storage) {
	defaultStorage = s
}

func (tx *Transaction) SetName(name string) *Transaction {
	tx.Name = name
	return tx
}

func (tx *Transaction) SetTimeout(timeout time.Duration) *Transaction {
	tx.Timeout = timeout
	return tx
}

func (tx *Transaction) storage() Storage {
	if defaultStorage == nil {
		panic("gtm: default storage is nil")
	}
	return defaultStorage
}

func (tx *Transaction) timer() Timer {
	if defaultTimer == nil {
		panic("gtm: default timer is nil")
	}
	return defaultTimer
}

func (tx *Transaction) doer() Doer {
	if defaultTimer == nil {
		panic("gtm: default doer is nil")
	}
	return defaultDoer
}

func (tx *Transaction) timeout() time.Duration {
	if tx.Timeout > 0 {
		return tx.Timeout
	}
	return defaultTimeout
}

func (tx *Transaction) AddNormal(partners ...NormalPartner) *Transaction {
	tx.NormalPartners = append(tx.NormalPartners, partners...)
	return tx
}

func (tx *Transaction) AddUncertain(partner UncertainPartner) *Transaction {
	tx.UncertainPartner = partner
	return tx
}

func (tx *Transaction) AddCertain(partners ...CertainPartner) *Transaction {
	tx.CertainPartners = append(tx.CertainPartners, partners...)
	return tx
}

func (tx *Transaction) AddAsync(partners ...CertainPartner) *Transaction {
	tx.AsyncPartners = append(tx.AsyncPartners, partners...)
	return tx
}

func (tx *Transaction) AddPartners(normalPartners []NormalPartner, uncertainPartner UncertainPartner, certainPartners, asyncPartners []CertainPartner) *Transaction {
	tx.NormalPartners = normalPartners
	tx.UncertainPartner = uncertainPartner
	tx.CertainPartners = certainPartners
	tx.AsyncPartners = asyncPartners

	return tx
}

// ExecuteAsync save the transaction only and will return immediately.
// The transaction will be executed asynchronously in the background.
func (tx *Transaction) ExecuteAsync() (err error) {
	tx.RetryAt = time.Now()
	tx.Timeout = tx.timeout()
	if _, err := tx.storage().SaveTransaction(tx); err != nil {
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
	tx.Times++
	retryTime := tx.timer().CalcRetryTime(tx.Times, tx.timeout())
	if err := tx.storage().UpdateTransactionRetryTime(tx, tx.Times, retryTime); err != nil {
		return Uncertain, fmt.Errorf("set transaction retry time err: %v", err)
	}

	return tx.execute()
}

// Execute will execute the transaction and return the execution result.
// The first step performed is to save the transaction data for redo.
// About the returned:
// 1. The returned results may be Success/Fail/Uncertain.
// 2. The returned err may not be nil when results is Fail/Uncertain.
// 3. When the result is Success/Fail, it means that the transaction has reached the final state.
func (tx *Transaction) Execute() (result Result, err error) {
	tx.Times = 1
	tx.RetryAt = tx.timer().CalcRetryTime(0, tx.timeout())
	tx.Timeout = tx.timeout()
	if tx.ID, err = tx.storage().SaveTransaction(tx); err != nil {
		return Fail, fmt.Errorf("save transaction failed: %v", err)
	}

	return tx.execute()
}

func (tx *Transaction) execute() (result Result, err error) {
	tx.StartAt = time.Now()

	result, undoOffset, err := tx.do()

	switch result {
	case Success:
		done, err := tx.doNext()
		if err != nil {
			return Uncertain, fmt.Errorf("doNext() failed: %v", err)
		}

		if done {
			if err := tx.saveResult(Success); err != nil {
				return Uncertain, fmt.Errorf("save result failed: %v, %v", err, Success)
			}
		}

		return Success, nil
	case Fail:
		if err := tx.undo(undoOffset); err != nil {
			return Uncertain, fmt.Errorf("undo() failed: %v", err)
		}

		if err := tx.saveResult(Fail); err != nil {
			return Uncertain, fmt.Errorf("save result failed: %v, %v", err, Fail)
		}

		return Fail, err
	default:
		return Uncertain, fmt.Errorf("do err: %v", err)
	}
}

// do is used to execute uncertain operations.
// Equivalent to the Prepare phase in 2PC.
// The result of Do is uncertain.
// If successful, DoNext will be executed; if it fails, Undo will be executed.
func (tx *Transaction) do() (result Result, undoOffset int, err error) {
	result, undoOffset, err = tx.doer().DoNormal(tx)
	if result != Success {
		return result, undoOffset, fmt.Errorf("doNormal failed: %v", err)
	}

	return tx.doer().DoUncertain(tx)
}

// doNext is used to supplement do.
// Equivalent to the Commit phase in 2PC.
// DoNext expects all results to be successful, otherwise it will try again.
func (tx *Transaction) doNext() (done bool, err error) {
	return tx.doer().DoNext(tx)
}

// undo will rollback all successful do.
// Equivalent to the Rollback phase in 2PC.
// Undo expects all results to be successful, otherwise it will try again.
func (tx *Transaction) undo(undoOffset int) error {
	return tx.doer().Undo(tx, undoOffset)
}

// saveResult saves the final result of the transaction.
// Cost is the sum of the execution time of each transaction.
func (tx *Transaction) saveResult(result Result) error {
	cost := time.Since(tx.StartAt)

	if err := tx.storage().SaveTransactionResult(tx, cost, Fail); err != nil {
		return fmt.Errorf("save transaction result failed: %v, %v, %v", err, cost, Fail)
	}

	return nil
}

// getPartnerResult returns the execution result of the partner at each phase.
// The transaction will not call storage for the first time to improve performance.
// Errors returned by Storage will be ignored for the transaction to continue.
func (tx *Transaction) getPartnerResult(phase string, offset int) (result Result) {
	if tx.Times <= 1 {
		return ""
	}

	var err error
	if result, err = tx.storage().GetPartnerResult(tx, phase, offset); err != nil {
		return ""
	}

	return result
}
