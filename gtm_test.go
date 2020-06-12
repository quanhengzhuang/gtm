package gtm

import (
	"log"
	"testing"
)

type AmountChanger struct {
	OrderID    int
	UserID     int
	Amount     int
	ChangeType int
	Remark     string
}

func (a *AmountChanger) Do() (Result, error) {
	log.Printf("amount do. id = %v, amount = %v", a.OrderID, a.Amount)
	return Success, nil
}

func (a *AmountChanger) DoNext() error {
	log.Printf("amount do next")
	return nil
}

func (a *AmountChanger) Undo() error {
	log.Printf("amount undo")
	return nil
}

type OrderCreator struct {
	OrderID   int
	UserID    int
	ProductID int
	Amount    int
}

func (order *OrderCreator) Do() (Result, error) {
	log.Printf("order do. id = %v, amount = %v", order.OrderID, order.Amount)
	return Success, nil
	// return false, fmt.Errorf("xxx")
}

func TestGtm(t *testing.T) {
	amount := AmountChanger{1990001, 10001, 99, 0, "test"}
	order := OrderCreator{1990001, 10001, 11, 99}

	gtm := New()
	gtm.AddPartners([]NormalPartner{&amount}, &order, nil)
	ok, err := gtm.Execute()
	if err != nil {
		t.Errorf("gtm execute failed: %v", err)
	}

	t.Errorf("gtm result = %v", ok)
}
