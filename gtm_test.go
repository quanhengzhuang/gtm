package gtm_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/quanhengzhuang/gtm"
)

func init() {
	storage := NewLevelStorage()
	// gob.RegisterName("*gtm.Payer", &Payer{})
	// gob.RegisterName("*gtm.OrderCreator", &OrderCreator{})
	storage.Register(&Payer{})
	storage.Register(&OrderCreator{})

	gtm.SetStorage(storage)

	db, err := gorm.Open("mysql", "root:root1234@/gtm?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}

	dbs := gtm.NewDBStorage(db)
	gtm.SetStorage(dbs)
}

func TestNew(t *testing.T) {
	tx := gtm.New()
	tx.AddNormalPartners(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})
	tx.AddUncertainPartner(&OrderCreator{OrderID: "100001", UserID: 20001, ProductID: 31, Amount: 99})

	switch result, err := tx.Execute(); result {
	case gtm.Success:
		t.Logf("tx's result = success")
	case gtm.Fail:
		t.Logf("tx's result = fail. err = %+v", err)
	case gtm.Uncertain:
		t.Logf("tx's result = uncertain: err = %v", err)
	}
}

func TestBackground(t *testing.T) {
	tx := gtm.New()
	tx.AddNormalPartners(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})
	tx.AddUncertainPartner(&OrderCreator{OrderID: "100001", UserID: 20001, ProductID: 31, Amount: 99})

	if err := tx.ExecuteBackground(); err != nil {
		log.Printf("execute background err = %v", err)
	}
}

func retry() error {
	count := 10

	for {
		transactions, results, errs, err := gtm.RetryTimeoutTransactions(count)
		if err != nil {
			return fmt.Errorf("retry err: %v", err)
		}

		for k, tx := range transactions {
			log.Printf("retry id = %v, result = %v, err = %v", tx.ID, results[k], errs[k])
		}

		if len(transactions) == 0 {
			time.Sleep(time.Minute)
		}
	}
}

func TestRetry(t *testing.T) {
	if err := retry(); err != nil {
		t.Errorf("retry failed: %v", err)
	}
}
