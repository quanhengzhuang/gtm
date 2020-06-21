package gtm

import (
	"fmt"
	"time"
)

type Doer interface {
	DoNormal(tx *Transaction) (result Result, undoOffset int, err error)
	DoUncertain(tx *Transaction) (result Result, undoOffset int, err error)
	DoNext(tx *Transaction) (done bool, err error)
	Undo(tx *Transaction, undoOffset int) (err error)
}

var (
	_ Doer = &SequenceDoer{}
)

// SequenceDoer is an sequentially executor.
// All methods of partner will be executed in the order of registration.
type SequenceDoer struct{}

func (*SequenceDoer) DoNormal(tx *Transaction) (result Result, undoOffset int, err error) {
	phase := "do-normal"

	for i, partner := range tx.NormalPartners {
		if result = tx.getPartnerResult(phase, i); result == "" {
			begin := time.Now()
			result, err = partner.Do()
			if err := tx.storage().SavePartnerResult(tx, phase, i, time.Since(begin), result); err != nil {
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

func (*SequenceDoer) DoUncertain(tx *Transaction) (result Result, undoOffset int, err error) {
	if tx.UncertainPartner == nil {
		return Success, 0, nil
	}

	phase := "do-uncertain"

	if result = tx.getPartnerResult(phase, 0); result == "" {
		begin := time.Now()
		result, err = tx.UncertainPartner.Do()
		if result == Success || result == Fail {
			if err := tx.storage().SavePartnerResult(tx, phase, 0, time.Since(begin), result); err != nil {
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

func (*SequenceDoer) DoNext(tx *Transaction) (done bool, err error) {
	var partners []CertainPartner
	for _, v := range tx.NormalPartners {
		partners = append(partners, v)
	}

	partners = append(partners, tx.CertainPartners...)

	if tx.Times > 1 {
		partners = append(partners, tx.AsyncPartners...)
		done = true
	}

	phase := "doNext"

	for i, v := range partners {
		if result := tx.getPartnerResult(phase, i); result != Success {
			begin := time.Now()
			if err = v.DoNext(); err != nil {
				return done, fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage().SavePartnerResult(tx, phase, i, time.Since(begin), Success); err != nil {
				return done, fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return done, nil
}

func (*SequenceDoer) Undo(tx *Transaction, undoOffset int) (err error) {
	phase := "undo"

	for i := undoOffset; i >= 0; i-- {
		if result := tx.getPartnerResult(phase, i); result != Success {
			begin := time.Now()
			if err := tx.NormalPartners[i].Undo(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage().SavePartnerResult(tx, phase, i, time.Since(begin), Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}
