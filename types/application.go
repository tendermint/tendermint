package types

type Application interface {

	// For new socket connections
	Open() AppContext
}

type AppContext interface {

	// Echo a message
	Echo(message string) string

	// Return application info
	Info() []string

	// Set application option (e.g. mode=mempool, mode=consensus)
	SetOption(key string, value string) RetCode

	// Append a tx, which may or may not get committed
	AppendTx(tx []byte) ([]Event, RetCode)

	// Return the application Merkle root hash
	GetHash() ([]byte, RetCode)

	// Set commit checkpoint
	Commit() RetCode

	// Rollback to the latest commit
	Rollback() RetCode

	// Add event listener
	AddListener(key string) RetCode

	// Remove event listener
	RemListener(key string) RetCode

	// Close this AppContext
	Close() error
}
