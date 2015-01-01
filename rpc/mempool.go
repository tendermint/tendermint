package rpc

import (
	"bytes"
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
	tx := block.TxDecoder(reader, n, &err).(block.Tx)
	if err != nil {
		ReturnJSON(API_INVALID_PARAM, Fmt("Invalid tx_bytes: %v", err))
	}

	err = mempoolReactor.BroadcastTx(tx)
	if err != nil {
		ReturnJSON(API_ERROR, Fmt("Error broadcasting transaction: %v", err))
	}

	ReturnJSON(API_OK, Fmt("Broadcasted tx: %X", tx))
}

/*
curl --data 'tx_bytes=0101146070FF17C39B2B0A64CA2BC431328037FA0F476064000000000000000001407D28F5CEE2065FCB2952CA9B99E9F9855E992B0FA5862442F582F2A84C3B3B31154A86D54DD548AFF080697BDC15AF26E68416AA678EF29449BB8D273B73320502206BD490C212E701A2136EEEA04F06FA4F287EE47E2B7A9B5D62EDD84CD6AD975301146070FF17C39B2B0A64CA2BC431328037FA0F47606400000000000000' -H 'content-type: text/plain;' http://127.0.0.1:8888/mempool
tx: 0101146070FF17C39B2B0A64CA2BC431328037FA0F476064000000000000000001407D28F5CEE2065FCB2952CA9B99E9F9855E992B0FA5862442F582F2A84C3B3B31154A86D54DD548AFF080697BDC15AF26E68416AA678EF29449BB8D273B73320502206BD490C212E701A2136EEEA04F06FA4F287EE47E2B7A9B5D62EDD84CD6AD975301146070FF17C39B2B0A64CA2BC431328037FA0F47606400000000000000
*/
