//go:build gofuzz || go1.18

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tendermint/tendermint/libs/log"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func FuzzRPCJSONRPCServer(f *testing.F) {
	type args struct {
		S string `json:"s"`
		I int    `json:"i"`
	}
	var rpcFuncMap = map[string]*rpcserver.RPCFunc{
		"c": rpcserver.NewRPCFunc(func(context.Context, *args) (string, error) {
			return "foo", nil
		}),
	}

	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, rpcFuncMap, log.NewNopLogger())
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		req, err := http.NewRequest("POST", "http://localhost/", bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		res := rec.Result()
		blob, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
		if len(blob) == 0 {
			return
		}

		if outputJSONIsSlice(blob) {
			var recv []types.RPCResponse
			if err := json.Unmarshal(blob, &recv); err != nil {
				panic(err)
			}
			return
		}
		var recv types.RPCResponse
		if err := json.Unmarshal(blob, &recv); err != nil {
			panic(err)
		}
	})
}

func outputJSONIsSlice(input []byte) bool {
	var slice []json.RawMessage
	return json.Unmarshal(input, &slice) == nil
}
