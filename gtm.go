package gtm

import (
	"fmt"
)

type GTM struct {
	partners []Partner
}

type Partner interface {
	Prepare() (bool, error)
	Commit() error
	Rollback() error
}

func New() *GTM {
	return &GTM{}
}

func (g *GTM) AddPartner(partner Partner) {
	g.partners = append(g.partners, partner)
}

func (g *GTM) Execute() (ok bool, err error) {
	if failOffset, err := g.prepareAll(); err != nil {
		if err := g.rollbackAll(failOffset); err != nil {
			return false, fmt.Errorf("rollback failed: %v", err)
		} else {
			return false, nil
		}
	} else {
		if err := g.commitAll(); err != nil {
			return true, fmt.Errorf("commit failed: %v", err)
		} else {
			return true, nil
		}
	}
}

func (g *GTM) prepareAll() (failOffset int, err error) {
	for k, v := range g.partners {
		if ok, err := v.Prepare(); err != nil {
			return k, fmt.Errorf("prepare failed: %v", err)
		} else if !ok {
			return k - 1, fmt.Errorf("prepare not ok")
		}
	}

	return 0, nil
}

func (g *GTM) rollbackAll(failOffset int) error {
	for i := 0; i <= failOffset; i++ {
		if err := g.partners[i].Rollback(); err != nil {
			return fmt.Errorf("rollback failed: %v", err)
		}
	}

	return nil
}

func (g *GTM) commitAll() error {
	for _, v := range g.partners {
		if err := v.Commit(); err != nil {
			return fmt.Errorf("commit failed: %v", err)
		}
	}

	return nil
}
