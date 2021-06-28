package mempool

import (
	"crypto/sha256"

	"github.com/tendermint/tendermint/types"
)

// TxKeySize defines the size of the transaction's key used for indexing.
const TxKeySize = sha256.Size

// TxKey is the fixed length array key used as an index.
func TxKey(tx types.Tx) [TxKeySize]byte {
	return sha256.Sum256(tx)
}

// TxHashFromBytes returns the hash of a transaction from raw bytes.
func TxHashFromBytes(tx []byte) []byte {
	return types.Tx(tx).Hash()
}

// TxInfo are parameters that get passed when attempting to add a tx to the
// mempool.
type TxInfo struct {
	// SenderID is the internal peer ID used in the mempool to identify the
	// sender, storing two bytes with each transaction instead of 20 bytes for
	// the types.NodeID.
	SenderID uint16

	// SenderNodeID is the actual types.NodeID of the sender.
	SenderNodeID types.NodeID
}
