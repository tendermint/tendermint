package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/tendermint/tendermint/libs/log"
	rs "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	types "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

var rpcFuncMap = map[string]*rs.RPCFunc{
	"c": rs.NewRPCFunc(func(s string, i int) (string, int) { return "foo", 200 }, "s,i", false),
}
var mux *http.ServeMux

func init() {
	mux = http.NewServeMux()
	lgr := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	rs.RegisterRPCFuncs(mux, rpcFuncMap, lgr)
}

func Fuzz(data []byte) int {
	if len(data) == 0 {
		return -1
	}

	req, _ := http.NewRequest("POST", "http://localhost/", bytes.NewReader(data))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	blob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if err := res.Body.Close(); err != nil {
		panic(err)
	}
	if len(blob) == 0 {
		return 1
	}

	if outputJSONIsSlice(blob) {
		recv := []types.RPCResponse{}
		if err := json.Unmarshal(blob, &recv); err != nil {
			panic(err)
		}
		return 1
	}
	recv := &types.RPCResponse{}
	if err := json.Unmarshal(blob, recv); err != nil {
		panic(err)
	}
	return 1
}

func outputJSONIsSlice(input []byte) bool {
	slice := []interface{}{}
	return json.Unmarshal(input, &slice) == nil
}
