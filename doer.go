package gtm

import (
	"fmt"
)

type Doer interface {
	DoNormal(tx *Transaction) (result Result, undoOffset int, err error)
	DoUncertain(tx *Transaction) (result Result, undoOffset int, err error)
	DoNext(tx *Transaction) (err error)
	Undo(tx *Transaction, undoOffset int) (err error)
}

type SequenceDoer struct{}

func (*SequenceDoer) DoNormal(tx *Transaction) (result Result, undoOffset int, err error) {
	phase := "do-normal"

	for i, partner := range tx.NormalPartners {
		if result = tx.getPartnerResult(phase, i); result == "" {
			result, err = partner.Do()
			if err := tx.storage().SavePartnerResult(tx, phase, i, result); err != nil {
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
		result, err = tx.UncertainPartner.Do()
		if result == Success || result == Fail {
			if err := tx.storage().SavePartnerResult(tx, phase, 0, result); err != nil {
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
func (*SequenceDoer) DoNext(tx *Transaction) (err error) {
	var partners []CertainPartner
	for _, v := range tx.NormalPartners {
		partners = append(partners, v)
	}
	partners = append(partners, tx.CertainPartners...)

	phase := "doNext"

	for i, v := range partners {
		if result := tx.getPartnerResult(phase, i); result == "" {
			if err = v.DoNext(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage().SavePartnerResult(tx, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}

func (*SequenceDoer) Undo(tx *Transaction, undoOffset int) (err error) {
	phase := "undo-normal"

	for i := undoOffset; i >= 0; i-- {
		if result := tx.getPartnerResult(phase, i); result == "" {
			if err := tx.NormalPartners[i].Undo(); err != nil {
				return fmt.Errorf("partner return err: %v, %v, %v", phase, i, err)
			}

			if err := tx.storage().SavePartnerResult(tx, phase, i, Success); err != nil {
				return fmt.Errorf("save partner result failed: %v, %v, %v", phase, i, err)
			}
		}
	}

	return nil
}
