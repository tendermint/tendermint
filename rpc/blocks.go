// Maybe move this to blocks/handler.go
package rpc

import (
	"net/http"
	//. "github.com/tendermint/tendermint/blocks"
)

func BlockHandler(w http.ResponseWriter, r *http.Request) {
	//height, _ := GetParamUint64Safe(r, "height")
	//count, _ := GetParamUint64Safe(r, "count")

	ReturnJSON(API_OK, "hello")
}
