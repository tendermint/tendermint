package rpc

import (
	"net/http"
	//. "github.com/tendermint/tendermint/block"
)

func BlockHandler(w http.ResponseWriter, r *http.Request) {
	//height, _ := GetParamUint64Safe(r, "height")
	//count, _ := GetParamUint64Safe(r, "count")

	ReturnJSON(API_OK, "hello")
}
