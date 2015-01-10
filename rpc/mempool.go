package rpc

import (
	"net/http"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

func BroadcastTxHandler(w http.ResponseWriter, r *http.Request) {
	txJSON := GetParam(r, "tx")
	var err error
	var tx Tx
	ReadJSON(&tx, []byte(txJSON), &err)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid tx: %v", err))
		return
	}

	err = mempoolReactor.BroadcastTx(tx)
	if err != nil {
		WriteAPIResponse(w, API_ERROR, Fmt("Error broadcasting transaction: %v", err))
		return
	}

	WriteAPIResponse(w, API_OK, "")
	return
}

/*
curl -H 'content-type: text/plain;' http://127.0.0.1:8888/submit_tx?tx=...
*/
