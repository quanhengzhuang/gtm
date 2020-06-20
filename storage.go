package gtm

import (
	"time"
)

type Storage interface {
	// Save the transaction data.
	// Must be reliable.
	// Return a unique transaction ID.
	SaveTransaction(tx *Transaction) (id string, err error)

	// Save the execution result of the transaction.
	// Must be reliable.
	SaveTransactionResult(tx *Transaction, result Result) error

	// Save the execution result of partner.
	// Performance first, not necessarily reliable.
	SavePartnerResult(tx *Transaction, phase string, offset int, cost time.Duration, result Result) error

	// Return partner's result
	GetPartnerResult(tx *Transaction, phase string, offset int) (Result, error)

	// UpdateTransactionRetryTime use to change the transaction's retryTime.
	UpdateTransactionRetryTime(tx *Transaction, times int, newRetryTime time.Time) error

	// Return transactions to be retried.
	GetTimeoutTransactions(count int) ([]*Transaction, error)
}
