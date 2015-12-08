package rpcclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/rpc/types"
)

func CallHTTP(remote string, method string, params []interface{}, dest interface{}) (interface{}, error) {
	// Make request and get responseBytes
	request := rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      "",
	}
	requestBytes := wire.JSONBytes(request)
	requestBuf := bytes.NewBuffer(requestBytes)
	log.Info(Fmt("RPC request to %v: %v", remote, string(requestBytes)))
	httpResponse, err := http.Post(remote, "text/json", requestBuf)
	if err != nil {
		return dest, err
	}
	defer httpResponse.Body.Close()
	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return dest, err
	}
	log.Info(Fmt("RPC response: %v", string(responseBytes)))

	// Parse response into JSONResponse
	response := rpctypes.RPCResponse{}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return dest, err
	}
	// Parse response into dest
	resultJSONObject := response.Result
	errorStr := response.Error
	if errorStr != "" {
		return dest, errors.New(errorStr)
	}
	dest = wire.ReadJSONObject(dest, resultJSONObject, &err)
	return dest, err
}
