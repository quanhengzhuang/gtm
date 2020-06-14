package gtm

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

var (
	_ NormalPartner    = &Payer{}
	_ UncertainPartner = &OrderCreator{}
)

type Payer struct {
	OrderID string
	UserID  int
	Amount  int
}

func (p *Payer) Do() (Result, error) {
	log.Printf("[payer] lock money. p = %+v", p)
	return Success, nil
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

func (order *OrderCreator) Do() (Result, error) {
	log.Printf("[order] create order. order = %+v", order)

	rand.Seed(time.Now().UnixNano())
	switch rand.Int() % 3 {
	case 0:
		return Success, nil
	case 1:
		return Fail, fmt.Errorf("understock")
	default:
		return Uncertain, fmt.Errorf("network anomaly")
	}
}
