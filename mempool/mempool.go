package mempool

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

// Mempool defines the mempool interface.
//
// Updates to the mempool need to be synchronized with committing a block so
// apps can reset their transient state on Commit.
type Mempool interface {
	// CheckTx executes a new transaction against the application to determine
	// its validity and whether it should be added to the mempool.
	CheckTx(tx types.Tx, callback func(*abci.Response)) error

	// CheckTxWithInfo performs the same operation as CheckTx, but with extra
	// meta data about the tx.
	// Currently this metadata is the peer who sent it, used to prevent the tx
	// from being gossiped back to them.
	CheckTxWithInfo(tx types.Tx, callback func(*abci.Response), txInfo TxInfo) error

	// ReapMaxBytesMaxGas reaps transactions from the mempool up to maxBytes
	// bytes total with the condition that the total gasWanted must be less than
	// maxGas.
	// If both maxes are negative, there is no cap on the size of all returned
	// transactions (~ all available transactions).
	ReapMaxBytesMaxGas(maxBytes, maxGas int64) types.Txs

	// ReapMaxTxs reaps up to max transactions from the mempool.
	// If max is negative, there is no cap on the size of all returned
	// transactions (~ all available transactions).
	ReapMaxTxs(max int) types.Txs

	// Lock locks the mempool. The consensus must be able to hold lock to safely update.
	Lock()

	// Unlock unlocks the mempool.
	Unlock()

	// Update informs the mempool that the given txs were committed and can be discarded.
	// NOTE: this should be called *after* block is committed by consensus.
	// NOTE: unsafe; Lock/Unlock must be managed by caller
	Update(blockHeight int64, blockTxs types.Txs, deliverTxResponses []*abci.ResponseDeliverTx, newPreFn PreCheckFunc, newPostFn PostCheckFunc) error

	// FlushAppConn flushes the mempool connection to ensure async reqResCb calls are
	// done. E.g. from CheckTx.
	FlushAppConn() error

	// Flush removes all transactions from the mempool and cache
	Flush()

	// TxsAvailable returns a channel which fires once for every height,
	// and only when transactions are available in the mempool.
	// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
	TxsAvailable() <-chan struct{}

	// EnableTxsAvailable initializes the TxsAvailable channel, ensuring it will
	// trigger once every height when transactions are available.
	EnableTxsAvailable()

	// Size returns the number of transactions in the mempool.
	Size() int

	// TxsBytes returns the total size of all txs in the mempool.
	TxsBytes() int64

	// InitWAL creates a directory for the WAL file and opens a file itself.
	InitWAL()

	// CloseWAL closes and discards the underlying WAL file.
	// Any further writes will not be relayed to disk.
	CloseWAL()
}

//--------------------------------------------------------------------------------

// PreCheckFunc is an optional filter executed before CheckTx and rejects
// transaction if false is returned. An example would be to ensure that a
// transaction doesn't exceeded the block size.
type PreCheckFunc func(types.Tx) error

// PostCheckFunc is an optional filter executed after CheckTx and rejects
// transaction if false is returned. An example would be to ensure a
// transaction doesn't require more gas than available for the block.
type PostCheckFunc func(types.Tx, *abci.ResponseCheckTx) error

// TxInfo are parameters that get passed when attempting to add a tx to the
// mempool.
type TxInfo struct {
	// We don't use p2p.ID here because it's too big. The gain is to store max 2
	// bytes with each tx to identify the sender rather than 20 bytes.
	SenderID uint16
}

//--------------------------------------------------------------------------------

// PreCheckAminoMaxBytes checks that the size of the transaction plus the amino
// overhead is smaller or equal to the expected maxBytes.
func PreCheckAminoMaxBytes(maxBytes int64) PreCheckFunc {
	return func(tx types.Tx) error {
		// We have to account for the amino overhead in the tx size as well
		// NOTE: fieldNum = 1 as types.Block.Data contains Txs []Tx as first field.
		// If this field order ever changes this needs to updated here accordingly.
		// NOTE: if some []Tx are encoded without a parenting struct, the
		// fieldNum is also equal to 1.
		aminoOverhead := types.ComputeAminoOverhead(tx, 1)
		txSize := int64(len(tx)) + aminoOverhead
		if txSize > maxBytes {
			return fmt.Errorf("Tx size (including amino overhead) is too big: %d, max: %d",
				txSize, maxBytes)
		}
		return nil
	}
}

// PostCheckMaxGas checks that the wanted gas is smaller or equal to the passed
// maxGas. Returns nil if maxGas is -1.
func PostCheckMaxGas(maxGas int64) PostCheckFunc {
	return func(tx types.Tx, res *abci.ResponseCheckTx) error {
		if maxGas == -1 {
			return nil
		}
		if res.GasWanted < 0 {
			return fmt.Errorf("gas wanted %d is negative",
				res.GasWanted)
		}
		if res.GasWanted > maxGas {
			return fmt.Errorf("gas wanted %d is greater than max gas %d",
				res.GasWanted, maxGas)
		}
		return nil
	}
}
