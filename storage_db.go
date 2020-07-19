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

// DBStorage is a GTM Storage implementation using DB.
// It depends on a gorm.DB.
type DBStorage struct {
	db *gorm.DB
}

// NewDBStorage returns a *DBStorage and needs to be injected into the gorm.DB.
func NewDBStorage(db *gorm.DB) *DBStorage {
	return &DBStorage{db: db}
}

/*
DROP TABLE gtm_transactions;

CREATE TABLE gtm_transactions (
	id         bigint UNSIGNED NOT NULL AUTO_INCREMENT,
	name       varchar(50) NOT NULL,
	times      int UNSIGNED NOT NULL,
	retry_at   timestamp NOT NULL,
	timeout    int UNSIGNED NOT NULL,
	result     enum('success', 'fail', '') NOT NULL,
	cost       bigint UNSIGNED NOT NULL,
	content    mediumtext,
	created_at timestamp NOT NULL,
	updated_at timestamp NOT NULL,

	PRIMARY KEY (id),
	KEY idx_retry (result, retry_at)
);
*/
type DBStorageTransaction struct {
	ID        int
	Name      string
	Times     int
	RetryAt   time.Time
	Timeout   int
	Result    string
	Cost      time.Duration
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (*DBStorageTransaction) TableName() string {
	return "gtm_transactions"
}

/*
DROP TABLE gtm_partner_result;

CREATE TABLE gtm_partner_result (
	id              bigint UNSIGNED NOT NULL AUTO_INCREMENT,
	transaction_id  bigint UNSIGNED NOT NULL,
	phase           varchar(20) NOT NULL,
	offset          tinyint UNSIGNED NOT NULL,
	result          enum('success', 'fail', 'uncertain') NOT NULL,
	cost            bigint UNSIGNED NOT NULL,
	created_at      timestamp NOT NULL,
	updated_at      timestamp NOT NULL,

	PRIMARY KEY (id),
	UNIQUE KEY uni_tx_id (transaction_id, phase, offset)
);
*/
type DBStoragePartnerResult struct {
	ID            int
	TransactionID int
	Offset        int
	Phase         string
	Result        string
	Cost          time.Duration
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (*DBStoragePartnerResult) TableName() string {
	return "gtm_partner_result"
}

// SaveTransaction save transaction data to db.
func (s *DBStorage) SaveTransaction(tx *Transaction) (id string, err error) {
	var content string
	if content, err = s.encode(tx); err != nil {
		return "", fmt.Errorf("encode err: %v", err)
	}

	data := DBStorageTransaction{
		Name:    tx.Name,
		Times:   tx.Times,
		RetryAt: tx.RetryAt,
		Timeout: int(tx.Timeout.Seconds()),
		Content: content,
	}

	if err := s.db.Create(&data).Error; err != nil {
		return "", fmt.Errorf("db create failed: %v", err)
	}

	return strconv.Itoa(data.ID), nil
}

// SaveTransactionResult save transaction results to db.
func (s *DBStorage) SaveTransactionResult(tx *Transaction, cost time.Duration, result Result) error {
	if err := s.db.Model(DBStorageTransaction{}).Where("id=?", tx.ID).Update(map[string]interface{}{
		"cost":   int64(cost),
		"result": result,
	}).Error; err != nil {
		return fmt.Errorf("update err: %v", err)
	}

	return nil
}

// SavePartnerResult save the result of a phase of partner to db.
func (s *DBStorage) SavePartnerResult(tx *Transaction, phase string, offset int, cost time.Duration, result Result) error {
	txID, err := strconv.Atoi(tx.ID)
	if err != nil {
		return fmt.Errorf("strconv id err: %v", err)
	}

	data := DBStoragePartnerResult{
		TransactionID: txID,
		Phase:         phase,
		Offset:        offset,
		Cost:          cost,
		Result:        string(result),
	}

	if err := s.db.Create(&data).Error; err != nil {
		return fmt.Errorf("db create failed: %v", err)
	}

	return nil
}

// GetPartnerResult returns the execution result of a partner.
func (s *DBStorage) GetPartnerResult(tx *Transaction, phase string, offset int) (Result, error) {
	var row DBStoragePartnerResult
	if err := s.db.Where("transaction_id=? AND phase=? AND offset=?", tx.ID, phase, offset).
		Find(&row).Error; err != nil {
		return "", fmt.Errorf("find err: %v", err)
	}

	return Result(row.Result), nil
}

// UpdateTransactionRetryTime update transaction next retry time.
func (s *DBStorage) UpdateTransactionRetryTime(tx *Transaction, times int, newRetryTime time.Time) error {
	data := map[string]interface{}{
		"times":    times,
		"retry_at": newRetryTime,
	}

	if err := s.db.Model(DBStorageTransaction{}).Where("id=?", tx.ID).Update(data).Error; err != nil {
		return fmt.Errorf("update err: %v", err)
	}

	return nil
}

// GetTimeoutTransactions returns all transactions that require timeout retry.
func (s *DBStorage) GetTimeoutTransactions(count int) (txs []*Transaction, err error) {
	var rows []DBStorageTransaction
	err = s.db.Where("result=? AND retry_at<?", "", time.Now()).Limit(count).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find err: %v", err)
	}

	for _, row := range rows {
		tx, err := s.decode(row.Content)
		if err != nil {
			return nil, fmt.Errorf("tx decode err: %v", err)
		}

		tx.ID = strconv.Itoa(row.ID)
		tx.Times = row.Times

		txs = append(txs, tx)
	}

	return txs, nil
}

func (s *DBStorage) Register(values ...interface{}) {
	for _, value := range values {
		gob.Register(value)
	}
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
