package gtm

import (
	"fmt"
)

// GTM is the definition of global transaction manager.
// Including multiple NormalPartners, one UncertainPartner, multiple CertainPartners.
type GTM struct {
	ID   int
	Name string

	NormalPartners   []NormalPartner
	UncertainPartner UncertainPartner
	CertainPartners  []CertainPartner
	AsyncPartners    []CertainPartner

	storage Storage
}

type Result string

const (
	Success   Result = "success"
	Fail      Result = "fail"
	Uncertain Result = "uncertain"

	DoNextRetrying Result = "doNextRetrying"
	UndoRetrying   Result = "undoRetrying"
)

var (
	defaultStorage Storage
)

// NormalPartner is a normal participant.
// This participant needs three methods to implement 2PC.
// In business, DoNext is often omitted and can directly return success.
// If Do returns a failure, Undo will not be executed, because gtm thinks there is no impact.
// If Do returns uncertainty, Undo will be executed.
type NormalPartner interface {
	Do() (Result, error)
	DoNext() error
	Undo() error
}

// UncertainPartner is an uncertain (unstable) participant.
// The execution result only accepts success and failure, and the result will be retried if uncertain.
// Only one participant of this type is allowed in each gtm transaction.
type UncertainPartner interface {
	Do() (Result, error)
}

// CertainPartner is a certain (stable) participant.
// The execution result can only be success, other results will be retried.
// This type of participant is placed at the end of the gtm transaction, allowing multiple.
type CertainPartner interface {
	DoNext() error
}

func New() *GTM {
	if defaultStorage == nil {
		panic("default storage is nil")
	}

	return &GTM{
		ID:      defaultStorage.GenerateID(),
		storage: defaultStorage,
	}
}

func (g *GTM) SetName(name string) *GTM {
	g.Name = name
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

func (g *GTM) ExecuteBackground() {

}

func (g *GTM) Execute() (result Result, err error) {
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
			_ = g.storage.SaveTransactionResult(g.ID, DoNextRetrying)
			return DoNextRetrying, fmt.Errorf("doNext() failed: %v", err)
		}

		_ = g.storage.SaveTransactionResult(g.ID, Success)
		return Success, nil
	case Fail:
		if err := g.undo(undoOffset); err != nil {
			_ = g.storage.SaveTransactionResult(g.ID, UndoRetrying)
			return UndoRetrying, fmt.Errorf("undo() failed: %v", err)
		}

		_ = g.storage.SaveTransactionResult(g.ID, Fail)
		return Fail, g.undo(undoOffset)
	case Uncertain:
		return Uncertain, fmt.Errorf("do err: %v", err)
	default:
		panic("unexpect Execute()'s result: " + result)
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
	for current, partner := range g.NormalPartners {
		result, err := partner.Do()
		_ = g.storage.SavePartnerResult(g.ID, "normal-do", current, result)

		switch result {
		case Fail:
			return Fail, current - 1, fmt.Errorf("do's failed: %v", err)
		case Uncertain:
			return Fail, current, fmt.Errorf("do's uncertain: %v", err)
		}
	}

	return Success, 0, nil
}

func (g *GTM) doUncertain() (result Result, undoOffset int, err error) {
	if g.UncertainPartner != nil {
		result, err := g.UncertainPartner.Do()

		switch result {
		case Success:
			_ = g.storage.SavePartnerResult(g.ID, "uncertain-do", 0, result)
		case Fail:
			_ = g.storage.SavePartnerResult(g.ID, "uncertain-do", 0, result)
			return Fail, len(g.NormalPartners) - 1, fmt.Errorf("do's failed: %v", err)
		case Uncertain:
			return Uncertain, 0, fmt.Errorf("uncertain partner do err: %v", err)
		}
	}

	return Success, 0, nil
}

// doNext is used to supplement do.
// Equivalent to the Commit phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (g *GTM) doNext() error {
	for _, v := range g.NormalPartners {
		if err := v.DoNext(); err != nil {
			return fmt.Errorf("partner's DoNext() failed: %v", err)
		}
		_ = g.storage.SavePartnerResult(g.ID, "normal-doNext", 0, Success)
	}

	for _, v := range g.CertainPartners {
		if err := v.DoNext(); err != nil {
			return fmt.Errorf("partner's DoNext() failed: %v", err)
		}
		_ = g.storage.SavePartnerResult(g.ID, "certain-doNext", 0, Success)
	}

	return nil
}

// undo will rollback all successful do.
// Equivalent to the Rollback phase in 2PC.
// Failure is not allowed at this phase and will be retried.
func (g *GTM) undo(undoOffset int) error {
	for i := 0; i <= undoOffset; i++ {
		if err := g.NormalPartners[i].Undo(); err != nil {
			return fmt.Errorf("partner's Undo() failed: %v", err)
		}
		_ = g.storage.SavePartnerResult(g.ID, "undo", 0, Success)
	}

	return nil
}
