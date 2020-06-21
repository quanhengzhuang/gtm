package gtm_test

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/quanhengzhuang/gtm"
	"log"
	"testing"
)

func init() {
	db, err := gorm.Open("mysql", "root:root1234@/gtm?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	db.LogMode(true)

	s := gtm.NewDBStorage(db)
	s.Register(&Payer{}, &OrderCreator{})

	gtm.SetStorage(s)
}

func TestNew(t *testing.T) {
	tx := gtm.New()
	tx.AddNormalPartners(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})
	tx.AddUncertainPartner(&OrderCreator{OrderID: "100001", UserID: 20001, ProductID: 31, Amount: 99})
	tx.AddAsyncPartners(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})

	switch result, err := tx.Execute(); result {
	case gtm.Success:
		t.Logf("tx id = %v, result = success", tx.ID)
	case gtm.Fail:
		t.Logf("tx id = %v, result = fail. err = %+v", tx.ID, err)
	default:
		t.Logf("tx id = %v, result = %v: err = %v", tx.ID, result, err)
	}
}

func TestAsync(t *testing.T) {
	for i := 0; i < 10; i++ {
		tx := gtm.New()
		tx.AddNormalPartners(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})
		tx.AddUncertainPartner(&OrderCreator{OrderID: "100001", UserID: 20001, ProductID: 31, Amount: 99})

		if err := tx.ExecuteAsync(); err != nil {
			t.Errorf("execute background err = %v", err)
		}
	}
}

func TestRetry(t *testing.T) {
	count := 10
	transactions, results, errs, err := gtm.RetryTimeoutTransactions(count)
	if err != nil {
		t.Errorf("retry err: %v", err)
	}

	for k, tx := range transactions {
		t.Logf("retry id = %v, result = %v, err = %v", tx.ID, results[k], errs[k])
	}
}
