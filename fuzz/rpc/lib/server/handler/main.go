package handler

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	rs "github.com/tendermint/tendermint/rpc/lib/server"
	types "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tmlibs/log"
)

var rpcFuncMap = map[string]*rs.RPCFunc{
	"c": rs.NewRPCFunc(func(s string, i int) (string, int) { return "foo", 200 }, "s,i"),
}

func Fuzz(data []byte) int {
	mux := http.NewServeMux()
	buf := new(bytes.Buffer)
	lgr := log.NewTMLogger(buf)
	rs.RegisterRPCFuncs(mux, rpcFuncMap, lgr)

	req, _ := http.NewRequest("POST", "http://localhost/", bytes.NewReader(data))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	blob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	recv := new(types.RPCResponse)
	if err := json.Unmarshal(blob, recv); err != nil {
		panic(err)
	}
	return 1
}