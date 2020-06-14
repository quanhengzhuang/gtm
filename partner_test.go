package gtm_test

import (
	"fmt"
	"github.com/quanhengzhuang/gtm"
	"log"
	"math/rand"
	"time"
)

var (
	_ gtm.NormalPartner    = &Payer{}
	_ gtm.UncertainPartner = &OrderCreator{}
)

type Payer struct {
	OrderID string
	UserID  int
	Amount  int
}

func (p *Payer) Do() (gtm.Result, error) {
	log.Printf("[payer] lock money. p = %+v", p)
	return gtm.Success, nil
}

func (p *Payer) DoNext() error {
	log.Printf("[payer] deduct lock. p = %+v", p)
	return nil
}

func (p *Payer) Undo() error {
	log.Printf("[payer] unlock money. p = %+v", p)
	return nil
}

type OrderCreator struct {
	OrderID   int
	UserID    int
	ProductID int
	Amount    int
}

func (order *OrderCreator) Do() (gtm.Result, error) {
	log.Printf("[order] create order. order = %+v", order)

	rand.Seed(time.Now().UnixNano())
	switch rand.Int() % 3 {
	case 0:
		return gtm.Success, nil
	case 1:
		return gtm.Fail, fmt.Errorf("understock")
	default:
		return gtm.Uncertain, fmt.Errorf("network anomaly")
	}
}
