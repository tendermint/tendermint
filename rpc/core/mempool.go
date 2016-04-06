package core

import (
	"fmt"
	ctypes "github.com/eris-ltd/tendermint/rpc/core/types"
	"github.com/eris-ltd/tendermint/state"
	"github.com/eris-ltd/tendermint/types"
)

//-----------------------------------------------------------------------------

// Note: tx must be signed
func BroadcastTx(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempoolReactor.BroadcastTx(tx)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}

	txHash := types.TxID(mempoolReactor.Mempool.GetState().ChainID, tx)
	var createsContract uint8
	var contractAddr []byte
	// check if creates new contract
	if callTx, ok := tx.(*types.CallTx); ok {
		if len(callTx.Address) == 0 {
			createsContract = 1
			contractAddr = state.NewContractAddress(callTx.Input.Address, callTx.Input.Sequence)
		}
	}
	return &ctypes.ResultBroadcastTx{ctypes.Receipt{txHash, createsContract, contractAddr}}, nil
}

func ListUnconfirmedTxs() (*ctypes.ResultListUnconfirmedTxs, error) {
	txs := mempoolReactor.Mempool.GetProposalTxs()
	return &ctypes.ResultListUnconfirmedTxs{len(txs), txs}, nil
}
