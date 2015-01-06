package rpc

import (
	"bytes"
	"fmt"
	"net/http"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

func MempoolHandler(w http.ResponseWriter, r *http.Request) {
	//height, _ := GetParamUint64Safe(r, "height")
	//count, _ := GetParamUint64Safe(r, "count")
	txBytes, err := GetParamByteSlice(r, "tx_bytes")
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid tx_bytes: %v", err))
		return
	}

	reader, n := bytes.NewReader(txBytes), new(int64)
	tx_ := ReadBinary(struct{ Tx }{}, reader, n, &err).(struct{ Tx })
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid tx_bytes: %v", err))
		return
	}
	tx := tx_.Tx

	err = mempoolReactor.BroadcastTx(tx)
	if err != nil {
		WriteAPIResponse(w, API_ERROR, Fmt("Error broadcasting transaction: %v", err))
		return
	}

	jsonBytes := JSONBytes(tx)
	fmt.Println(">>", string(jsonBytes))

	WriteAPIResponse(w, API_OK, Fmt("Broadcasted tx: %X", tx))
	return
}

/*
curl -H 'content-type: text/plain;' http://127.0.0.1:8888/mempool?tx_bytes=0101146070FF17C39B2B0A64CA2BC431328037FA0F4760640000000000000001014025BE0AD9344DA24BC12FCB84903F88AB9B82107F414A0B570048CDEF7EDDD5AC96B34F0405B7A75B761447F8E6C899343F1EDB160B4C4FAAE92D5881D0023A0701206BD490C212E701A2136EEEA04F06FA4F287EE47E2B7A9B5D62EDD84CD6AD975301146070FF17C39B2B0A64CA2BC431328037FA0F47FF6400000000000000
*/
