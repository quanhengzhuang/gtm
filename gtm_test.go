package gtm

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"log"
	"testing"
	"time"
)

type LocalStorage struct {
	db *leveldb.DB
}

func NewLocalStorage() *LocalStorage {
	db, err := leveldb.OpenFile("gtm_data", nil)
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}

	return &LocalStorage{db}
}

func (s *LocalStorage) Register(value interface{}) {
	gob.Register(value)
}

func (s *LocalStorage) SaveTransaction(g *GTM) (id int, err error) {
	g.ID = int(time.Now().UnixNano())

	// retry key
	retry := fmt.Sprintf("gtm-retry-%v", g.ID)
	if err := s.db.Put([]byte(retry), []byte(fmt.Sprintf("%v", g.ID)), nil); err != nil {
		return 0, fmt.Errorf("db put failed: %v", err)
	}
	log.Printf("[storage] put retry key: %v", retry)

	// transaction
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(g); err != nil {
		return 0, fmt.Errorf("gob encode err: %v", err)
	}

	key := fmt.Sprintf("gtm-transaction-%v", g.ID)
	if err := s.db.Put([]byte(key), buffer.Bytes(), nil); err != nil {
		return 0, fmt.Errorf("db put failed: %v", err)
	}

	return g.ID, nil
}

func (s *LocalStorage) SaveTransactionResult(id int, result Result) error {
	key := fmt.Sprintf("gtm-result-%v", id)
	if err := s.db.Put([]byte(key), []byte(result), nil); err != nil {
		return fmt.Errorf("db put failed: %v", err)
	}

	// delete retry
	if result == Success || result == Fail {
		retry := fmt.Sprintf("gtm-retry-%v", id)
		if err := s.db.Delete([]byte(retry), nil); err != nil {
			return fmt.Errorf("delete retry err: %v", err)
		}
		log.Printf("[storage] delete retry key: %v", retry)
	}

	return nil
}

func (s *LocalStorage) SavePartnerResult(id int, phase string, offset int, result Result) error {
	log.Printf("[storage] save partner result. id:%v, phase:%v, offset:%v, result:%v", id, phase, offset, result)

	key := fmt.Sprintf("gtm-partner-%v-%v-%v", id, phase, offset)
	if err := s.db.Put([]byte(key), []byte(result), nil); err != nil {
		return fmt.Errorf("db put failed: %v", err)
	}

	return nil
}

func (s *LocalStorage) SetTransactionRetryTime(id int, times int, retryTime time.Time) error {
	return nil
}

func (s *LocalStorage) GetTimeoutTransactions(count int) (transactions []*GTM, err error) {
	ids := [][]byte{}

	iterator := s.db.NewIterator(util.BytesPrefix([]byte("gtm-retry-")), nil)
	for i := 0; i < count && iterator.Next(); i++ {
		ids = append(ids, iterator.Value())
	}
	iterator.Release()

	for _, id := range ids {
		value, err := s.db.Get([]byte(fmt.Sprintf("gtm-transaction-%s", id)), nil)
		if err != nil {
			return nil, fmt.Errorf("get transaction err: %v", err)
		}

		var g GTM
		if err := gob.NewDecoder(bytes.NewReader(value)).Decode(&g); err != nil {
			return nil, fmt.Errorf("gob decode err: %v", err)
		}
		transactions = append(transactions, &g)
	}

	return transactions, nil
}

func (s *LocalStorage) GetPartnerResult(id int, phase string, offset int) (Result, error) {
	return "", nil
}

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

func init() {
	storage := NewLocalStorage()
	storage.Register(&AmountChanger{})
	storage.Register(&OrderCreator{})
	SetStorage(storage)
}

func TestRetry(t *testing.T) {
	transactions, err := GetTimeoutTransactions(100)
	t.Logf("get timeout transactions: %v", len(transactions))

	if err != nil {
		t.Errorf("get timeout transactions err: %v", err)
	}

	for _, tx := range transactions {
		log.Printf("id = %v, retryTime = %v", tx.ID, tx.RetryTime)
		if _, err := tx.ExecuteContinue(); err != nil {
			t.Errorf("tx execute continue err: %v", err)
		}
	}
}

func TestNew(t *testing.T) {
	amount := AmountChanger{1990001, 10001, 99, 0, "test"}
	order := OrderCreator{1990001, 10001, 11, 99}

	// new
	gtm := New()
	gtm.AddPartners([]NormalPartner{&amount}, &order, nil)
	ok, err := gtm.Execute()
	if err != nil {
		t.Errorf("gtm Execute() err: %v", err)
	}

	t.Logf("gtm result = %v", ok)
}
