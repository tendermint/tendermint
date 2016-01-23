package types

type Application interface {

	// Echo a message
	Echo(message string) string

	// Return application info
	Info() []string

	// Set application option (e.g. mode=mempool, mode=consensus)
	SetOption(key string, value string) RetCode

	// Append a tx
	AppendTx(tx []byte) ([]Event, RetCode)

	// Validate a tx for the mempool
	CheckTx(tx []byte) RetCode

	// Return the application Merkle root hash
	GetHash() ([]byte, RetCode)

	// Add event listener
	AddListener(key string) RetCode

	// Remove event listener
	RemListener(key string) RetCode

	// Query for state
	Query(query []byte) ([]byte, RetCode)
}
