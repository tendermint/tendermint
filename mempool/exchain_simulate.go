package mempool

import abci "github.com/tendermint/tendermint/abci/types"

// SimulationResponse defines the response generated when a transaction is successfully
// simulated by the Baseapp.
type SimulationResponse struct {
	GasInfo
	Result *Result
}

// GasInfo defines tx execution gas context.
type GasInfo struct {
	// GasWanted is the maximum units of work we allow this tx to perform.
	GasWanted uint64

	// GasUsed is the amount of gas actually consumed.
	GasUsed uint64
}

// Result is the union of ResponseFormat and ResponseCheckTx.
type Result struct {
	// Data is any data returned from message or handler execution. It MUST be length
	// prefixed in order to separate data from multiple message executions.
	Data []byte

	// Log contains the log information from message or handler execution.
	Log string

	// Events contains a slice of Event objects that were emitted during message or
	// handler execution.
	Events []abci.Event
}
