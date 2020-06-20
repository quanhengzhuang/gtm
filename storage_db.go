package gtm

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
)

var (
	_ Storage = &DBStorage{}
)

/*
CREATE TABLE gtm_transactions (
	id         bigint UNSIGNED NOT NULL AUTO_INCREMENT,
	name       varchar(50) NOT NULL,
	times      int UNSIGNED NOT NULL,
	retry_time timestamp NOT NULL,
	timeout    int UNSIGNED NOT NULL,
	result     varchar(20) NOT NULL,
	content    mediumtext,
	create_at  timestamp NOT NULL,
	update_at  timestamp NOT NULL,

	PRIMARY KEY (id),
	KEY idx_retry (result, retry_time)
);
*/
type DBStorageTransaction struct {
	ID        int
	Name      string
	Times     int
	RetryTime time.Time
	Timeout   int
	Result    string
	Content   string
	CreateAt  time.Time
	UpdateAt  time.Time
}

func (t *DBStorageTransaction) TableName() string {
	return "gtm_transactions"
}

/*
CREATE TABLE gtm_partner_result (
	id             bigint UNSIGNED NOT NULL AUTO_INCREMENT,
	tx_id          bigint UNSIGNED NOT NULL,
	phase          varchar(20) NOT NULL,
	offset         tinyint UNSIGNED NOT NULL,
	result         varchar(20) NOT NULL,
	cost           int UNSIGNED NOT NULL,
	create_at      timestamp NOT NULL,
	update_at      timestamp NOT NULL,

	PRIMARY KEY (id),
	UNIQUE KEY uni_tid (tx_id, phase, offset)
);
*/
type DBStoragePartnerResult struct {
	ID       int
	TxID     int
	Offset   int
	Phase    string
	Result   string
	Cost     int
	CreateAt time.Time
	UpdateAt time.Time
}

func (r *DBStoragePartnerResult) TableName() string {
	return "gtm_partner_result"
}

type DBStorage struct {
	db *gorm.DB
}

func NewDBStorage(db *gorm.DB) *DBStorage {
	return &DBStorage{db: db}
}

func (s *DBStorage) SaveTransaction(tx *Transaction) (id string, err error) {
	var content string
	if content, err = s.encode(tx); err != nil {
		return "", fmt.Errorf("encode err: %v", err)
	}

	data := DBStorageTransaction{
		Name:      tx.Name,
		Times:     tx.Times,
		RetryTime: tx.RetryTime,
		Timeout:   int(tx.Timeout.Seconds()),
		Content:   content,
	}

	if err := s.db.Create(&data).Error; err != nil {
		return "", fmt.Errorf("db create failed: %v", err)
	}

	return fmt.Sprintf("%v", data.ID), nil
}

func (s *DBStorage) SaveTransactionResult(tx *Transaction, result Result) error {
	if err := s.db.Model(DBStorageTransaction{}).Where("id=?", tx.ID).Update(map[string]interface{}{
		"result": result,
	}).Error; err != nil {
		return fmt.Errorf("update err: %v", err)
	}

	return nil
}

func (s *DBStorage) SavePartnerResult(tx *Transaction, phase string, offset int, result Result) error {
	txID, err := strconv.Atoi(tx.ID)
	if err != nil {
		return fmt.Errorf("strconv id err: %v", err)
	}

	data := DBStoragePartnerResult{
		TxID:   txID,
		Phase:  phase,
		Offset: offset,
		Result: string(result),
	}

	if err := s.db.Create(&data).Error; err != nil {
		return fmt.Errorf("db create failed: %v", err)
	}

	return nil
}

func (s *DBStorage) GetPartnerResult(tx *Transaction, phase string, offset int) (Result, error) {
	var result DBStoragePartnerResult
	if err := s.db.Where("tx_id=? AND phase=? AND offset=?", tx.ID, phase, offset).
		Find(&result).Error; err != nil {
		return "", fmt.Errorf("find err: %v", err)
	}

	return Result(result.Result), nil
}

func (s *DBStorage) UpdateTransactionRetryTime(tx *Transaction, times int, newRetryTime time.Time) error {
	if err := s.db.Model(DBStorageTransaction{}).Where("id=?", tx.ID).Update(map[string]interface{}{
		"times":      times,
		"retry_time": newRetryTime,
	}).Error; err != nil {
		return fmt.Errorf("update err: %v", err)
	}

	return nil
}

func (s *DBStorage) GetTimeoutTransactions(count int) (txs []*Transaction, err error) {
	var data []DBStorageTransaction
	if err := s.db.Where("result=? AND retry_time<?", "", time.Now()).
		Limit(count).Find(&data).Error; err != nil {
		return nil, fmt.Errorf("find err: %v", err)
	}

	for _, v := range data {
		tx, err := s.decode(v.Content)
		if err != nil {
			return nil, fmt.Errorf("tx decode err: %v", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

func (s *DBStorage) encode(tx *Transaction) (string, error) {
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(tx); err != nil {
		return "", fmt.Errorf("gob encode err: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func (s *DBStorage) decode(content string) (*Transaction, error) {
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("base64 decode err :%v", err)
	}

	var tx Transaction
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&tx); err != nil {
		return nil, fmt.Errorf("gob decode err: %v", err)
	}

	return &tx, nil
}
