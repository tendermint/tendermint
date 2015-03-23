package rpc

import (
	"net/http"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func BroadcastTxHandler(w http.ResponseWriter, r *http.Request) {
	txJSON := GetParam(r, "tx")
	var err error
	var tx types.Tx
	binary.ReadJSON(&tx, []byte(txJSON), &err)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid tx: %v", err))
		return
	}

	err = mempoolReactor.BroadcastTx(tx)
	if err != nil {
		WriteAPIResponse(w, API_ERROR, Fmt("Error broadcasting transaction: %v", err))
		return
	}

	txHash := merkle.HashFromBinary(tx)
	var createsContract bool
	var contractAddr []byte

	if callTx, ok := tx.(*types.CallTx); ok {
		if callTx.Address == nil {
			createsContract = true
			contractAddr = state.NewContractAddress(callTx.Input.Address, uint64(callTx.Input.Sequence))
		}
	}

	WriteAPIResponse(w, API_OK, struct {
		TxHash          []byte
		CreatesContract bool
		ContractAddr    []byte
	}{txHash, createsContract, contractAddr})
	return
}

/*
curl -H 'content-type: text/plain;' http://127.0.0.1:8888/submit_tx?tx=...
*/
