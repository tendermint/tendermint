package mempool

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

// Mempool defines the mempool interface.
// Updates to the mempool need to be synchronized with committing a block so
// apps can reset their transient state on Commit.
type Mempool interface {
	Lock()
	Unlock()

	Size() int

	CheckTx(types.Tx, func(*abci.Response)) error
	CheckTxWithInfo(types.Tx, func(*abci.Response), TxInfo) error

	ReapMaxBytesMaxGas(maxBytes, maxGas int64) types.Txs

	Update(int64, types.Txs, PreCheckFunc, PostCheckFunc) error
	Flush()
	FlushAppConn() error

	TxsAvailable() <-chan struct{}
	EnableTxsAvailable()
}

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
	PeerID uint16
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
