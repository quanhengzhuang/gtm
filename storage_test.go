package gtm_test

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/quanhengzhuang/gtm"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelStorage struct {
	db *leveldb.DB
}

func NewLevelStorage() *LevelStorage {
	db, err := leveldb.OpenFile("level_db_storage", nil)
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}

	return &LevelStorage{db}
}

func (s *LevelStorage) Register(value interface{}) {
	gob.Register(value)
}

func (s *LevelStorage) SaveTransaction(g *gtm.GTM) (id int, err error) {
	g.ID = int(time.Now().UnixNano())

	// add retry key
	retry := s.getRetryKey(g.RetryTime, g.ID)
	if err := s.db.Put([]byte(retry), []byte(fmt.Sprintf("%v", g.ID)), nil); err != nil {
		return 0, fmt.Errorf("db put failed: %v", err)
	}
	log.Printf("[storage] put retry key: %s", retry)

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

func (s *LevelStorage) SaveTransactionResult(id int, result gtm.Result) error {
	key := fmt.Sprintf("gtm-result-%v", id)
	if err := s.db.Put([]byte(key), []byte(result), nil); err != nil {
		return fmt.Errorf("db put failed: %v", err)
	}

	// delete retry
	if result == gtm.Success || result == gtm.Fail {
		retry := fmt.Sprintf("gtm-retry-%v", id)
		if err := s.db.Delete([]byte(retry), nil); err != nil {
			return fmt.Errorf("delete retry err: %v", err)
		}
		log.Printf("[storage] delete retry key: %v", retry)
	}

	return nil
}

func (s *LevelStorage) SavePartnerResult(id int, phase string, offset int, result gtm.Result) error {
	log.Printf("[storage] save partner result. id:%v, phase:%v, offset:%v, result:%v", id, phase, offset, result)

	key := fmt.Sprintf("gtm-partner-%v-%v-%v", id, phase, offset)
	if err := s.db.Put([]byte(key), []byte(result), nil); err != nil {
		return fmt.Errorf("db put failed: %v", err)
	}

	return nil
}

func (s *LevelStorage) SetTransactionRetryTime(g *gtm.GTM, times int, newRetryTime time.Time) error {
	// add new retry key
	key := s.getRetryKey(newRetryTime, g.ID)
	value := []byte(fmt.Sprintf("%v", g.ID))
	if err := s.db.Put(key, value, nil); err != nil {
		return fmt.Errorf("put err: %v", err)
	}

	// delete old retry key
	oldKey := s.getRetryKey(g.RetryTime, g.ID)
	if err := s.db.Delete(oldKey, nil); err != nil {
		return fmt.Errorf("delete err: %v", err)
	}
	log.Printf("[storage] set retry key. new:%s, old:%s", key, oldKey)

	return nil
}

func (s *LevelStorage) getRetryKey(retryTime time.Time, id int) []byte {
	return []byte(fmt.Sprintf("gtm-retry-%v-%v", retryTime.Unix(), id))
}

func (s *LevelStorage) GetTimeoutTransactions(count int) (transactions []*gtm.GTM, err error) {
	var ids [][]byte

	iterator := s.db.NewIterator(util.BytesPrefix([]byte("gtm-retry-")), nil)
	for count > 0 && iterator.Next() {
		key := bytes.Split(iterator.Key(), []byte("-"))
		if len(key) < 4 {
			continue
		}

		timeUnix, _ := strconv.Atoi(string(key[2]))
		if int64(timeUnix) > time.Now().Unix() {
			break
		}

		ids = append(ids, key[3])
		count--
	}

	iterator.Release()

	for _, id := range ids {
		value, err := s.db.Get([]byte(fmt.Sprintf("gtm-transaction-%s", id)), nil)
		if err != nil {
			return nil, fmt.Errorf("get transaction err: %v", err)
		}

		var g gtm.GTM
		if err := gob.NewDecoder(bytes.NewReader(value)).Decode(&g); err != nil {
			return nil, fmt.Errorf("gob decode err: %v", err)
		}
		transactions = append(transactions, &g)
	}

	return transactions, nil
}

func (s *LevelStorage) GetPartnerResult(id int, phase string, offset int) (gtm.Result, error) {
	key := fmt.Sprintf("gtm-partner-%v-%v-%v", id, phase, offset)
	if value, err := s.db.Get([]byte(key), nil); err != nil {
		return "", fmt.Errorf("db put failed: %v", err)
	} else {
		return gtm.Result(value), nil
	}
}

func init() {
	var _ gtm.Storage = &LevelStorage{}
}
