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
	tx_ := block.TxDecoder(reader, n, &err)
	if err != nil {
		ReturnJSON(API_INVALID_PARAM, Fmt("Invalid tx_bytes: %v", err))
	}
	// XXX Oops, I need to cast later like this, everywhere.
	tx := tx_.(block.Tx)

	err = mempoolReactor.BroadcastTx(tx)
	if err != nil {
		ReturnJSON(API_ERROR, Fmt("Error broadcasting transaction: %v", err))
	}

	ReturnJSON(API_OK, Fmt("Broadcasted tx: %X", tx))
}

/*
curl -H 'content-type: text/plain;' http://127.0.0.1:8888/mempool?tx_bytes=0101146070FF17C39B2B0A64CA2BC431328037FA0F47606400000000000000000140D209A7CD4E2E7C5E4B17815AB93029960AF66D3428DE7B085EBDBACD84A31F58562EFF0AC4EC7151B071DE82417110C94FFEE862A3740624D7A8C1874AFCF50402206BD490C212E701A2136EEEA04F06FA4F287EE47E2B7A9B5D62EDD84CD6AD975301146070FF17C39B2B0A64CA2BC431328037FA0F47FF6400000000000000
*/
