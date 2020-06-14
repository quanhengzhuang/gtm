package gtm

import (
	"time"
)

type Storage interface {
	// Save the transaction data.
	// Must be reliable.
	// Return a unique transaction ID.
	SaveTransaction(g *GTM) (id string, err error)

	// Save the execution result of the transaction.
	// Must be reliable.
	SaveTransactionResult(id string, result Result) error

	// Save the execution result of partner.
	// Performance first, not necessarily reliable.
	SavePartnerResult(id string, phase string, offset int, result Result) error

	// Return partner's result
	GetPartnerResult(id string, phase string, offset int) (Result, error)

	// Set transaction's retryTime.
	SetTransactionRetryTime(tx *GTM, times int, newRetryTime time.Time) error

	// Return transactions to be retried.
	GetTimeoutTransactions(count int) ([]*GTM, error)
}
