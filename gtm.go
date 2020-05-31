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
	// todo rename to execute
	Prepare() (bool, error)
	// todo rename to on success
	Commit() error
	Rollback() error
}

type UncertainPartner interface {
	Prepare() (bool, error)
}

type CertainPartner interface {
	Commit() error
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
	if ok, failOffset, err := g.prepareAll(); err != nil {
		if ok == Unknown {
			return Unknown, fmt.Errorf("prepare err: %v", err)
		}

		if err := g.rollbackAll(failOffset); err != nil {
			return Fail, fmt.Errorf("rollback failed: %v", err)
		} else {
			return Fail, nil
		}
	} else {
		if err := g.commitAll(); err != nil {
			return Success, fmt.Errorf("commit failed: %v", err)
		} else {
			return Success, nil
		}
	}
}

func (g *GTM) prepareAll() (result int, failOffset int, err error) {
	for k, v := range g.normalPartners {
		if ok, err := v.Prepare(); err != nil {
			return Fail, k, fmt.Errorf("prepare err: %v", err)
		} else if !ok {
			return Fail, k - 1, fmt.Errorf("prepare failed")
		}
	}

	if g.uncertainPartner != nil {
		if ok, err := g.uncertainPartner.Prepare(); err != nil {
			return Unknown, len(g.normalPartners) - 1, fmt.Errorf("uncertain partner prepare err: %v", err)
		} else if !ok {
			return Fail, len(g.normalPartners) - 1, fmt.Errorf("prepare failed")
		}
	}

	return Success, 0, nil
}

func (g *GTM) rollbackAll(failOffset int) error {
	for i := 0; i <= failOffset; i++ {
		if err := g.normalPartners[i].Rollback(); err != nil {
			return fmt.Errorf("rollback failed: %v", err)
		}
	}

	return nil
}

func (g *GTM) commitAll() error {
	for _, v := range g.normalPartners {
		if err := v.Commit(); err != nil {
			return fmt.Errorf("commit failed: %v", err)
		}
	}

	for _, v := range g.certainPartners {
		if err := v.Commit(); err != nil {
			return fmt.Errorf("commit failed: %v", err)
		}
	}

	return nil
}
