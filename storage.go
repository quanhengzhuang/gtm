package gtm

import (
	"time"
)

type Storage interface {
	// Save the transaction data.
	// Must be reliable.
	// Return a unique transaction ID.
	SaveTransaction(g *GTM) (id int, err error)

	// Save the execution result of the transaction.
	// Must be reliable.
	SaveTransactionResult(id int, result Result) error

	// Save the execution result of partner.
	// Performance first, not necessarily reliable.
	SavePartnerResult(id int, phase string, offset int, result Result) error

	// Return partner's result
	GetPartnerResult(id int, phase string, offset int) (Result, error)

	// Set transaction's retryTime.
	SetTransactionRetryTime(id int, times int, retryTime time.Time) error

	// Return transactions to be retried.
	GetTimeoutTransactions(count int) ([]*GTM, error)
}
