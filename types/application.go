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

// Some applications can choose to implement BlockchainAware
type BlockchainAware interface {

	// Initialize blockchain
	// validators: genesis validators from TendermintCore
	InitChain(validators []*Validator)

	// Signals the end of a block
	// validators: changed validators from app to TendermintCore
	EndBlock(height uint64) (validators []*Validator)
}
