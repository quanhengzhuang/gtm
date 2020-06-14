package gtm

import (
	"fmt"
	"time"
)

// GTM is the definition of global transaction manager.
// Including multiple NormalPartners, one UncertainPartner, multiple CertainPartners.
type GTM struct {
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

func New() *GTM {
	return &GTM{
		storage: defaultStorage,
		timer:   defaultTimer,
	}
}

func (g *GTM) SetName(name string) *GTM {
	g.Name = name
	return g
}

func (g *GTM) SetTimeout(timeout time.Duration) *GTM {
	g.Timeout = timeout
	return g
}

func SetStorage(storage Storage) {
	defaultStorage = storage
}

func (g *GTM) AddNormalPartners(partners ...NormalPartner) *GTM {
	g.NormalPartners = append(g.NormalPartners, partners...)
	return g
}

func (g *GTM) AddUncertainPartner(partner UncertainPartner) *GTM {
	g.UncertainPartner = partner
	return g
}

func (g *GTM) AddCertainPartners(partners ...CertainPartner) *GTM {
	g.CertainPartners = append(g.CertainPartners, partners...)
	return g
}

func (g *GTM) AddAsyncPartners(partners ...CertainPartner) *GTM {
	g.AsyncPartners = append(g.AsyncPartners, partners...)
	return g
}

// ExecuteBackground will return immediately.
func (g *GTM) ExecuteBackground() (err error) {
	if g.storage == nil {
		return fmt.Errorf("storage is nil")
	}

	g.RetryTime = g.timer.CalcRetryTime(0, g.Timeout)
	if _, err := g.storage.SaveTransaction(g); err != nil {
		return fmt.Errorf("save transaction failed: %v", err)
	}

	return nil
}

// RetryTimeoutTransactions retry to complete timeout transactions.
// Count is used to set the total number of transactions per retry.
// Returns the total number of actual retries, and retry errors.
func RetryTimeoutTransactions(count int) (transactions []*GTM, results []Result, errs []error, err error) {
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
func (g *GTM) ExecuteRetry() (result Result, err error) {
	// todo write once
	g.storage = defaultStorage
	g.timer = defaultTimer

	retryTime := g.timer.CalcRetryTime(g.Times+1, g.Timeout)
	if err := g.storage.SetTransactionRetryTime(g, g.Times+1, retryTime); err != nil {
		return Uncertain, fmt.Errorf("set transaction retry time err: %v", err)
	}

	return g.execute()
}

// Execute the transaction and return the final result.
func (g *GTM) Execute() (result Result, err error) {
	if g.storage == nil {
		return Fail, fmt.Errorf("storage is nil")
	}

	g.Times = 1
	g.RetryTime = g.timer.CalcRetryTime(0, g.Timeout)
	if g.ID, err = g.storage.SaveTransaction(g); err != nil {
		return Fail, fmt.Errorf("save transaction failed: %v", err)
	}

	return g.execute()
}

func (g *GTM) execute() (result Result, err error) {
	result, undoOffset, err := g.do()

	switch result {
	case Success:
		if err := g.doNext(); err != nil {
			_ = g.storage.SaveTransactionResult(g, doNextRetrying)
			return Uncertain, fmt.Errorf("doNext() failed: %v", err)
		}

		if err := g.storage.SaveTransactionResult(g, Success); err != nil {
			return Uncertain, fmt.Errorf("save transaction result failed: %v, %v", Success, err)
		}

		return Success, nil
	case Fail:
		if err := g.undo(undoOffset); err != nil {
			_ = g.storage.SaveTransactionResult(g, undoRetrying)
			return Uncertain, fmt.Errorf("undo() failed: %v", err)
		}

		if err := g.storage.SaveTransactionResult(g, Fail); err != nil {
			return Uncertain, fmt.Errorf("save transaction result failed: %v, %v", Fail, err)
		}

		return Fail, err
	default:
		return Uncertain, fmt.Errorf("do err: %v", err)
	}
}

// do is used to execute uncertain operations.
// Equivalent to the Prepare phase in 2PC.
func (g *GTM) do() (result Result, undoOffset int, err error) {
	result, undoOffset, err = g.doNormal()
	if result != Success {
		return result, undoOffset, fmt.Errorf("doNormal failed: %v", err)
	}

	return g.doUncertain()
}

func (g *GTM) doNormal() (result Result, undoOffset int, err error) {
	phase := "do-normal"

	for i, partner := range g.NormalPartners {
		if result = g.getPartnerResult(phase, i); result == "" {
			result, err = partner.Do()
			if err := g.storage.SavePartnerResult(g, phase, i, result); err != nil {
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

func (g *GTM) doUncertain() (result Result, undoOffset int, err error) {
	if g.UncertainPartner == nil {
		return Success, 0, nil
	}

	phase := "do-uncertain"

	if result = g.getPartnerResult(phase, 0); result == "" {
		result, err = g.UncertainPartner.Do()
		if result == Success || result == Fail {
			if err := g.storage.SavePartnerResult(g, phase, 0, result); err != nil {
				return Uncertain, 0, fmt.Errorf("save partner result failed: %v, %v, %v", phase, result, err)
			}
		}
	}

	switch result {
	case Success:
		return Success, 0, nil
	case Fail:
		return Fail, len(g.NormalPartners) - 1, fmt.Errorf("partner do failed: %v", err)
	case Uncertain:
		return Uncertain, 0, fmt.Errorf("partner return err: %v, %v, %v", phase, result, err)
	default:
		panic("unexpect result value: " + result)
	}
}

// doNext is used to supplement do.
// Equivalent to the Commit phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (g *GTM) doNext() error {
	if err := g.doNextNormal(); err != nil {
		return fmt.Errorf("normalPartner DoNext() failed: %v", err)
	}

	if err := g.doNextCertain(); err != nil {
		return fmt.Errorf("certainPartner DoNext() failed: %v", err)
	}

	return nil
}

func (g *GTM) doNextNormal() (err error) {
	phase := "doNext-normal"

	for i, v := range g.NormalPartners {
		if result := g.getPartnerResult(phase, i); result == "" {
			if err = v.DoNext(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := g.storage.SavePartnerResult(g, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

func (g *GTM) doNextCertain() error {
	phase := "doNext-certain"

	for i, v := range g.CertainPartners {
		if result := g.getPartnerResult(phase, i); result == "" {
			if err := v.DoNext(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := g.storage.SavePartnerResult(g, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

// undo will rollback all successful do.
// Equivalent to the Rollback phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (g *GTM) undo(undoOffset int) error {
	return g.undoNormal(undoOffset)
}

func (g *GTM) undoNormal(undoOffset int) error {
	phase := "undo-normal"

	for i := undoOffset; i >= 0; i-- {
		if result := g.getPartnerResult(phase, i); result == "" {
			if err := g.NormalPartners[i].Undo(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := g.storage.SavePartnerResult(g, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

func (g *GTM) getPartnerResult(phase string, offset int) (result Result) {
	if g.Times == 0 {
		return ""
	}

	var err error
	if result, err = g.storage.GetPartnerResult(g, phase, offset); err != nil {
		return ""
	}

	return result
}
