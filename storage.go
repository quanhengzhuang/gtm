package gtm

type Storage interface {
	// Return a unique transaction ID.
	GenerateID() int

	// Save the transaction data.
	// Must be reliable.
	SaveTransaction(g *GTM) error

	// Save the execution result of the transaction.
	// Must be reliable.
	SaveTransactionResult(id int, result Result) error

	// Save the execution result of partner.
	// Performance first, not necessarily reliable.
	SavePartnerResult(id int, offset int, result Result) error

	// Return transactions to be retried.
	GetUncertainTransactions(count int) ([]*GTM, error)
}
