package types

type Application interface {

	// Echo a message
	Echo(message string) string

	// Return application info
	Info() []string

	// Append a tx, which may or may not get committed
	AppendTx(tx []byte) RetCode

	// Return the application Merkle root hash
	GetHash() ([]byte, RetCode)

	// Set commit checkpoint
	Commit() RetCode

	// Rollback to the latest commit
	Rollback() RetCode

	// Set events reporting mode
	SetEventsMode(mode EventsMode) RetCode

	// Add event listener
	AddListener(key string) RetCode

	// Remove event listener
	RemListener(key string) RetCode

	// Get all events
	GetEvents() []Event
}
