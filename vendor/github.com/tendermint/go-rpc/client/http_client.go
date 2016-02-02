package rpcclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-rpc/types"
	"github.com/tendermint/go-wire"
)

// JSON rpc takes params as a slice
type ClientJSONRPC struct {
	remote string
}

func NewClientJSONRPC(remote string) *ClientJSONRPC {
	return &ClientJSONRPC{remote}
}

func (c *ClientJSONRPC) Call(method string, params []interface{}, result interface{}) (interface{}, error) {
	return CallHTTP_JSONRPC(c.remote, method, params, result)
}

// URI takes params as a map
type ClientURI struct {
	remote string
}

func NewClientURI(remote string) *ClientURI {
	if !strings.HasSuffix(remote, "/") {
		remote = remote + "/"
	}
	return &ClientURI{remote}
}

func (c *ClientURI) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	return CallHTTP_URI(c.remote, method, params, result)
}

func CallHTTP_JSONRPC(remote string, method string, params []interface{}, result interface{}) (interface{}, error) {
	// Make request and get responseBytes
	request := rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      "",
	}
	requestBytes := wire.JSONBytes(request)
	requestBuf := bytes.NewBuffer(requestBytes)
	log.Info(Fmt("RPC request to %v (%v): %v", remote, method, string(requestBytes)))
	httpResponse, err := http.Post(remote, "text/json", requestBuf)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	log.Info(Fmt("RPC response: %v", string(responseBytes)))
	return unmarshalResponseBytes(responseBytes, result)
}

func CallHTTP_URI(remote string, method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	values, err := argsToURLValues(params)
	if err != nil {
		return nil, err
	}
	log.Info(Fmt("URI request to %v (%v): %v", remote, method, values))
	resp, err := http.PostForm(remote+method, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return unmarshalResponseBytes(responseBytes, result)
}

//------------------------------------------------

func unmarshalResponseBytes(responseBytes []byte, result interface{}) (interface{}, error) {
	// read response
	// if rpc/core/types is imported, the result will unmarshal
	// into the correct type
	var err error
	response := &rpctypes.RPCResponse{}
	err = json.Unmarshal(responseBytes, response)
	if err != nil {
		return nil, err
	}
	errorStr := response.Error
	if errorStr != "" {
		return nil, errors.New(errorStr)
	}
	// unmarshal the RawMessage into the result
	result = wire.ReadJSONPtr(result, *response.Result, &err)
	return result, err
}

func argsToURLValues(args map[string]interface{}) (url.Values, error) {
	values := make(url.Values)
	if len(args) == 0 {
		return values, nil
	}
	err := argsToJson(args)
	if err != nil {
		return nil, err
	}
	for key, val := range args {
		values.Set(key, val.(string))
	}
	return values, nil
}

func argsToJson(args map[string]interface{}) error {
	var n int
	var err error
	for k, v := range args {
		buf := new(bytes.Buffer)
		wire.WriteJSON(v, buf, &n, &err)
		if err != nil {
			return err
		}
		args[k] = buf.String()
	}
	return nil
}
