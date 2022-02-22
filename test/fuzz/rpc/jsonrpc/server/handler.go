package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/tendermint/tendermint/libs/log"
	rs "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

var rpcFuncMap = map[string]*rs.RPCFunc{
	"c": rs.NewRPCFunc(func(ctx context.Context, s string, i int) (string, error) {
		return "foo", nil
	}, "s", "i"),
}
var mux *http.ServeMux

func init() {
	mux = http.NewServeMux()
	lgr := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	rs.RegisterRPCFuncs(mux, rpcFuncMap, lgr)
}

func Fuzz(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return errors.New("no data")
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/", bytes.NewReader(data))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if err := res.Body.Close(); err != nil {
		return err
	}
	if len(blob) == 0 {
		return nil
	}

	if outputJSONIsSlice(blob) {
		recv := []types.RPCResponse{}
		if err := json.Unmarshal(blob, &recv); err != nil {
			return err
		}
		return nil
	}
	recv := &types.RPCResponse{}
	if err := json.Unmarshal(blob, recv); err != nil {
		return err
	}
	return nil
}

func outputJSONIsSlice(input []byte) bool {
	slice := []interface{}{}
	return json.Unmarshal(input, &slice) == nil
}
