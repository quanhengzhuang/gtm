package gtm

import (
	"fmt"
)

type GTM struct {
	normalPartners   []NormalPartner
	uncertainPartner UncertainPartner
	certainPartners  []CertainPartner
}

const (
	Success = 0
	Fail    = 1
	Unknown = -1
)

type NormalPartner interface {
	Do() (bool, error)
	DoNext() error
	Undo() error
}

type UncertainPartner interface {
	Do() (bool, error)
}

type CertainPartner interface {
	DoNext() error
}

func New() *GTM {
	return &GTM{}
}

func (g *GTM) AddPartners(normal []NormalPartner, uncertain UncertainPartner, certain []CertainPartner) {
	g.normalPartners = normal
	g.uncertainPartner = uncertain
	g.certainPartners = certain
}

func (g *GTM) Execute() (result int, err error) {
	if ok, failOffset, err := g.do(); err != nil {
		if ok == Unknown {
			return Unknown, fmt.Errorf("do err: %v", err)
		}

		if err := g.undo(failOffset); err != nil {
			return Fail, fmt.Errorf("undo failed: %v", err)
		} else {
			return Fail, nil
		}
	} else {
		if err := g.doNext(); err != nil {
			return Success, fmt.Errorf("commit failed: %v", err)
		} else {
			return Success, nil
		}
	}
}

func (g *GTM) do() (result int, failOffset int, err error) {
	for k, v := range g.normalPartners {
		if ok, err := v.Do(); err != nil {
			return Fail, k, fmt.Errorf("do err: %v", err)
		} else if !ok {
			return Fail, k - 1, fmt.Errorf("do failed")
		}
	}

	if g.uncertainPartner != nil {
		if ok, err := g.uncertainPartner.Do(); err != nil {
			return Unknown, len(g.normalPartners) - 1, fmt.Errorf("uncertain partner do err: %v", err)
		} else if !ok {
			return Fail, len(g.normalPartners) - 1, fmt.Errorf("do failed")
		}
	}

	return Success, 0, nil
}

func (g *GTM) undo(failOffset int) error {
	for i := 0; i <= failOffset; i++ {
		if err := g.normalPartners[i].Undo(); err != nil {
			return fmt.Errorf("partner undo failed: %v", err)
		}
	}

	return nil
}

func (g *GTM) doNext() error {
	for _, v := range g.normalPartners {
		if err := v.DoNext(); err != nil {
			return fmt.Errorf("partner DoNext failed: %v", err)
		}
	}

	for _, v := range g.certainPartners {
		if err := v.DoNext(); err != nil {
			return fmt.Errorf("partner DoNext failed: %v", err)
		}
	}

	return nil
}
