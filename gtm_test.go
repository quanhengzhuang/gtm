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

func (a *AmountChanger) Prepare() (bool, error) {
	log.Printf("amount prepare. id = %v, amount = %v", a.OrderID, a.Amount)
	return true, nil
}

func (a *AmountChanger) Commit() error {
	log.Printf("amount commit")
	return nil
}

func (a *AmountChanger) Rollback() error {
	log.Printf("amount rollback")
	return nil
}

type OrderCreator struct {
	OrderID   int
	UserID    int
	ProductID int
	Amount    int
}

func (o *OrderCreator) Prepare() (bool, error) {
	log.Printf("order prepare. id = %v, amount = %v", o.OrderID, o.Amount)
	return false, nil
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
