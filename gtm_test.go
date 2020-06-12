package gtm

import (
	"bytes"
	"encoding/gob"
	"log"
	"testing"
)

type EmptyStorage struct{}

func (s *EmptyStorage) GenerateID() int {
	id := 202006130000001
	log.Printf("[storage] generate ID: %v", id)
	return id
}

func (s *EmptyStorage) SaveTransaction(g *GTM) (err error) {
	log.Printf("[storage] save transaction origin: %+v", g)

	var buffer bytes.Buffer
	gob.Register(&AmountChanger{})
	gob.Register(&OrderCreator{})

	err = gob.NewEncoder(&buffer).Encode(g)
	log.Printf("[storage] save transaction encode. err:%v, value:%+v", err, buffer)

	var g2 GTM
	err = gob.NewDecoder(&buffer).Decode(&g2)
	log.Printf("[storage] save transaction decode. err:%v, value:%+v, %+v", err, g2, g2.NormalPartners[0])

	return nil
}

func (s *EmptyStorage) SaveTransactionResult(id int, result Result) error {
	return nil
}

func (s *EmptyStorage) SavePartnerResult(id int, offset int, result Result) error {
	return nil
}

func (s *EmptyStorage) GetUncertainTransactions(count int) ([]*GTM, error) {
	return nil, nil
}

func init() {
	SetStorage(&EmptyStorage{})
}

func (a *AmountChanger) Do() (Result, error) {
	log.Printf("amount do. ID = %v, amount = %v", a.OrderID, a.Amount)
	return Success, nil
}

type AmountChanger struct {
	OrderID    int
	UserID     int
	Amount     int
	ChangeType int
	Remark     string
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

func TestGtm(t *testing.T) {
	amount := AmountChanger{1990001, 10001, 99, 0, "test"}
	order := OrderCreator{1990001, 10001, 11, 99}

	gtm := New()
	gtm.AddPartners([]NormalPartner{&amount}, &order, nil)
	ok, err := gtm.Execute()
	if err != nil {
		t.Errorf("gtm execute failed: %v", err)
	}

	t.Logf("gtm result = %v", ok)
}
