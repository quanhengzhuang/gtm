package gtm

import (
	"log"
)

var (
	_ NormalPartner    = &AmountChanger{}
	_ UncertainPartner = &OrderCreator{}
)

type AmountChanger struct {
	OrderID    int
	UserID     int
	Amount     int
	ChangeType int
	Remark     string
}

func (amount *AmountChanger) Do() (Result, error) {
	log.Printf("amount do. ID = %v, amount = %v", amount.OrderID, amount.Amount)
	return Success, nil
}

func (amount *AmountChanger) DoNext() error {
	log.Printf("amount do next")
	return nil
}

func (amount *AmountChanger) Undo() error {
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
	log.Printf("order do. ID = %v, amount = %v", order.OrderID, order.Amount)
	return Success, nil
}
