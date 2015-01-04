package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

func MempoolHandler(w http.ResponseWriter, r *http.Request) {
	//height, _ := GetParamUint64Safe(r, "height")
	//count, _ := GetParamUint64Safe(r, "count")
	txBytes, err := GetParamByteSlice(r, "tx_bytes")
	if err != nil {
		ReturnJSON(API_INVALID_PARAM, Fmt("Invalid tx_bytes: %v", err))
	}

	reader, n := bytes.NewReader(txBytes), new(int64)
	tx_ := block.TxDecoder(reader, n, &err)
	if err != nil {
		ReturnJSON(API_INVALID_PARAM, Fmt("Invalid tx_bytes: %v", err))
	}
	tx := tx_.(block.Tx)

	err = mempoolReactor.BroadcastTx(tx)
	if err != nil {
		ReturnJSON(API_ERROR, Fmt("Error broadcasting transaction: %v", err))
	}

	jsonBytes, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(">>", string(jsonBytes))

	ReturnJSON(API_OK, Fmt("Broadcasted tx: %X", tx))
}

/*
curl -H 'content-type: text/plain;' http://127.0.0.1:8888/mempool?tx_bytes=0101146070FF17C39B2B0A64CA2BC431328037FA0F4760640000000000000001014025BE0AD9344DA24BC12FCB84903F88AB9B82107F414A0B570048CDEF7EDDD5AC96B34F0405B7A75B761447F8E6C899343F1EDB160B4C4FAAE92D5881D0023A0701206BD490C212E701A2136EEEA04F06FA4F287EE47E2B7A9B5D62EDD84CD6AD975301146070FF17C39B2B0A64CA2BC431328037FA0F47FF6400000000000000
*/
