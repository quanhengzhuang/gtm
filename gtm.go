package gtm

import (
	"fmt"
	"time"
)

// GTM is the definition of global transaction manager.
// Including multiple NormalPartners, one UncertainPartner, multiple CertainPartners.
type GTM struct {
	ID   int
	Name string

	Times    int
	NextTime time.Time
	Timeout  time.Duration

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
		ID:      defaultStorage.GenerateID(),
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

func (g *GTM) GetID() int {
	return g.ID
}

func SetStorage(storage Storage) {
	defaultStorage = storage
}

func (g *GTM) AddPartners(normal []NormalPartner, uncertain UncertainPartner, certain []CertainPartner) *GTM {
	g.NormalPartners = normal
	g.UncertainPartner = uncertain
	g.CertainPartners = certain

	return g
}

func (g *GTM) AddAsyncPartners(certain []CertainPartner) *GTM {
	g.AsyncPartners = certain

	return g
}

// ExecuteBackground will return immediately.
func (g *GTM) ExecuteBackground() (err error) {
	if g.storage == nil {
		return fmt.Errorf("storage is nil")
	}

	if err := g.storage.SaveTransaction(g); err != nil {
		return fmt.Errorf("save transaction failed: %v", err)
	}

	return nil
}

// ExecuteContinue use to complete the transaction.
func (g *GTM) ExecuteContinue() (result Result, err error) {
	if g.storage == nil {
		return Uncertain, fmt.Errorf("storage is nil")
	}

	g.Times++
	g.NextTime = g.timer.CalcNextTime(g.Times, g.Timeout)

	// todo update the times and nextTime only
	if err := g.storage.SaveTransaction(g); err != nil {
		return Uncertain, fmt.Errorf("save transaction failed: %v", err)
	}

	return g.execute()
}

// Execute the transaction and return the final result.
func (g *GTM) Execute() (result Result, err error) {
	if g.storage == nil {
		return Fail, fmt.Errorf("storage is nil")
	}

	g.NextTime = g.timer.CalcNextTime(0, g.Timeout)
	if err := g.storage.SaveTransaction(g); err != nil {
		return Fail, fmt.Errorf("save transaction failed: %v", err)
	}

	return g.execute()
}

func (g *GTM) execute() (result Result, err error) {
	result, undoOffset, err := g.do()

	switch result {
	case Success:
		if err := g.doNext(); err != nil {
			_ = g.storage.SaveTransactionResult(g.ID, doNextRetrying)
			return Uncertain, fmt.Errorf("doNext() failed: %v", err)
		}

		_ = g.storage.SaveTransactionResult(g.ID, Success)
		return Success, nil
	case Fail:
		if err := g.undo(undoOffset); err != nil {
			_ = g.storage.SaveTransactionResult(g.ID, undoRetrying)
			return Uncertain, fmt.Errorf("undo() failed: %v", err)
		}

		_ = g.storage.SaveTransactionResult(g.ID, Fail)
		return Fail, g.undo(undoOffset)
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

	for current, partner := range g.NormalPartners {
		hasResult, result := g.hasPartnerResult(phase, current)
		if !hasResult {
			result, err = partner.Do()
			_ = g.storage.SavePartnerResult(g.ID, phase, current, result)
		}

		switch result {
		case Success:
			// continue
		case Fail:
			return Fail, current - 1, fmt.Errorf("do's failed: %v", err)
		case Uncertain:
			return Fail, current, fmt.Errorf("do's uncertain: %v", err)
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

	phase := "uncertain-do"

	hasResult, result := g.hasPartnerResult(phase, 0)
	if !hasResult {
		result, err = g.UncertainPartner.Do()
		if result == Success || result == Fail {
			_ = g.storage.SavePartnerResult(g.ID, phase, 0, result)
		}
	}

	switch result {
	case Success:
		return Success, 0, nil
	case Fail:
		return Fail, len(g.NormalPartners) - 1, fmt.Errorf("do's failed: %v", err)
	case Uncertain:
		return Uncertain, 0, fmt.Errorf("uncertain partner do err: %v", err)
	default:
		panic("unexpect result value: " + result)
	}
}

// doNext is used to supplement do.
// Equivalent to the Commit phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (g *GTM) doNext() error {
	if err := g.doNextNormal(); err != nil {
		return fmt.Errorf("partner's DoNext() failed: %v", err)
	}

	if err := g.doNextCertain(); err != nil {
		return fmt.Errorf("partner's DoNext() failed: %v", err)
	}

	return nil
}

func (g *GTM) doNextNormal() (err error) {
	phase := "doNext-normal"

	for current, v := range g.NormalPartners {
		hasResult, _ := g.hasPartnerResult(phase, current)
		if !hasResult {
			if err = v.DoNext(); err != nil {
				return fmt.Errorf("partner's DoNext() failed: %v", err)
			}
			_ = g.storage.SavePartnerResult(g.ID, phase, current, Success)
		}
	}

	return nil
}

func (g *GTM) doNextCertain() error {
	phase := "doNext-certain"

	for current, v := range g.CertainPartners {
		hasResult, _ := g.hasPartnerResult(phase, current)
		if !hasResult {
			if err := v.DoNext(); err != nil {
				return fmt.Errorf("partner's DoNext() failed: %v", err)
			}
			_ = g.storage.SavePartnerResult(g.ID, "certain-doNext", current, Success)
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
		hasResult, _ := g.hasPartnerResult(phase, i)
		if !hasResult {
			if err := g.NormalPartners[i].Undo(); err != nil {
				return fmt.Errorf("partner's Undo() failed: %v", err)
			}
			_ = g.storage.SavePartnerResult(g.ID, "undo", i, Success)
		}
	}

	return nil
}

func (g *GTM) hasPartnerResult(phase string, offset int) (has bool, result Result) {
	if g.Times == 0 {
		return false, ""
	}

	// todo storage get partner result

	return true, Success
}
