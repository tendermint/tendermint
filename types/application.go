package types

// Applications
type Application interface {

	// Return application info
	Info() (info string)

	// Set application option (e.g. mode=mempool, mode=consensus)
	SetOption(key string, value string) (log string)

	// Append a tx
	AppendTx(tx []byte) (code CodeType, result []byte, log string)

	// Validate a tx for the mempool
	CheckTx(tx []byte) (code CodeType, result []byte, log string)

	// Return the application Merkle root hash
	Commit() (hash []byte, log string)

	// Query for state
	Query(query []byte) (code CodeType, result []byte, log string)
}

// Some applications can choose to implement ValidatorAware
type ValidatorAware interface {

	// Give app initial list of validators upon genesis
	InitValidators([]*Validator)

	// Receive updates to validators from app, prior to commit
	SyncValidators() []*Validator
}
