package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

func Call(remote string, method string, params []interface{}, dest interface{}) (interface{}, error) {
	// Make request and get responseBytes
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		Id:      0,
	}
	requestBytes := binary.JSONBytes(request)
	requestBuf := bytes.NewBuffer(requestBytes)
	log.Debug(Fmt("RPC request to %v: %v", remote, string(requestBytes)))
	httpResponse, err := http.Post(remote, "text/json", requestBuf)
	if err != nil {
		return dest, err
	}
	defer httpResponse.Body.Close()
	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return dest, err
	}

	log.Debug(Fmt("RPC response: %v", string(responseBytes)))

	// Parse response into JSONResponse
	response := RPCResponse{}
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
	dest = binary.ReadJSONObject(dest, resultJSONObject, &err)
	return dest, err
}
