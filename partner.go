package gtm

// NormalPartner is a normal participant.
// This participant needs three methods to implement 2PC.
// In business, DoNext is often omitted and can directly return success.
// If Do returns a failure, Undo will not be executed, because gtm thinks there is no impact.
// If Do returns uncertainty, Undo will be executed.
type NormalPartner interface {
	Do() (Result, error)
	DoNext() error
	Undo() error
}

// UncertainPartner is an uncertain (unstable) participant.
// The execution result only accepts success and failure, and the result will be retried if uncertain.
// Only one participant of this type is allowed in each gtm transaction.
type UncertainPartner interface {
	Do() (Result, error)
}

// CertainPartner is a certain (stable) participant.
// The execution result can only be success, other results will be retried.
// This type of participant is placed at the end of the gtm transaction, allowing multiple.
type CertainPartner interface {
	DoNext() error
}
