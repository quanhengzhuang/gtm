package gtm

import (
	"fmt"
)

// GTM is the definition of global transaction manager.
// Including multiple normalPartners, one uncertainPartner, multiple certainPartners.
type GTM struct {
	name string

	normalPartners   []NormalPartner
	uncertainPartner UncertainPartner
	certainPartners  []CertainPartner
	asyncPartners    []CertainPartner
}

type Result string

const (
	Success   Result = "success"
	Fail      Result = "fail"
	Uncertain Result = "uncertain"
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
	return &GTM{}
}

func (g *GTM) SetName(name string) *GTM {
	g.name = name
	return g
}

func (g *GTM) AddPartners(normal []NormalPartner, uncertain UncertainPartner, certain []CertainPartner) *GTM {
	g.normalPartners = normal
	g.uncertainPartner = uncertain
	g.certainPartners = certain

	return g
}

func (g *GTM) AddAsyncPartners(certain []CertainPartner) *GTM {
	g.asyncPartners = certain

	return g
}

func (g *GTM) ExecuteBackground() {

}

func (g *GTM) Execute() (result Result, err error) {
	result, undoOffset, err := g.do()
	switch result {
	case Success:
		return Success, g.doNext()
	case Fail:
		return Fail, g.undo(undoOffset)
	case Uncertain:
		return Uncertain, fmt.Errorf("do err: %v", err)
	default:
		panic("unexpect Execute()'s result: " + result)
	}
}

func (g *GTM) do() (result Result, undoOffset int, err error) {
	for current, partner := range g.normalPartners {
		result, err := partner.Do()
		switch result {
		case Success:
			// success
		case Fail:
			return Fail, current - 1, fmt.Errorf("do's failed: %v", err)
		case Uncertain:
			return Fail, current, fmt.Errorf("do's uncertain: %v", err)
		default:
			panic("unexpect Do()'s result: " + result)
		}
	}

	if g.uncertainPartner != nil {
		result, err := g.uncertainPartner.Do()
		switch result {
		case Success:
			// success
		case Fail:
			return Fail, len(g.normalPartners) - 1, fmt.Errorf("do's failed: %v", err)
		case Uncertain:
			return Uncertain, 0, fmt.Errorf("uncertain partner do err: %v", err)
		default:
			panic("unexpect Do()'s result: " + result)
		}
	}

	return Success, 0, nil
}

func (g *GTM) undo(failOffset int) error {
	for i := 0; i <= failOffset; i++ {
		if err := g.normalPartners[i].Undo(); err != nil {
			return fmt.Errorf("partner's Undo() failed: %v", err)
		}
	}

	return nil
}

func (g *GTM) doNext() error {
	for _, v := range g.normalPartners {
		if err := v.DoNext(); err != nil {
			return fmt.Errorf("partner's DoNext() failed: %v", err)
		}
	}

	for _, v := range g.certainPartners {
		if err := v.DoNext(); err != nil {
			return fmt.Errorf("partner's DoNext() failed: %v", err)
		}
	}

	return nil
}
