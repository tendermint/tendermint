package rpcclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-rpc/types"
	"github.com/tendermint/go-wire"
)

// Set the net.Dial manually so we can do http over tcp or unix.
// Get/Post require a dummyDomain but it's over written by the Transport
var dummyDomain = "http://dummyDomain/"

func unixDial(remote string) func(string, string) (net.Conn, error) {
	return func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", remote)
	}
}

func tcpDial(remote string) func(string, string) (net.Conn, error) {
	return func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("tcp", remote)
	}
}

func socketTransport(remote string) *http.Transport {
	if rpctypes.SocketType(remote) == "unix" {
		return &http.Transport{
			Dial: unixDial(remote),
		}
	} else {
		return &http.Transport{
			Dial: tcpDial(remote),
		}
	}
}

//------------------------------------------------------------------------------------

// JSON rpc takes params as a slice
type ClientJSONRPC struct {
	remote string
	client *http.Client
}

func NewClientJSONRPC(remote string) *ClientJSONRPC {
	return &ClientJSONRPC{
		remote: remote,
		client: &http.Client{Transport: socketTransport(remote)},
	}
}

func (c *ClientJSONRPC) Call(method string, params []interface{}, result interface{}) (interface{}, error) {
	return c.call(method, params, result)
}

func (c *ClientJSONRPC) call(method string, params []interface{}, result interface{}) (interface{}, error) {
	// Make request and get responseBytes
	request := rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      "",
	}
	requestBytes := wire.JSONBytes(request)
	requestBuf := bytes.NewBuffer(requestBytes)
	log.Info(Fmt("RPC request to %v (%v): %v", c.remote, method, string(requestBytes)))
	httpResponse, err := c.client.Post(dummyDomain, "text/json", requestBuf)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	// 	log.Info(Fmt("RPC response: %v", string(responseBytes)))
	return unmarshalResponseBytes(responseBytes, result)
}

//-------------------------------------------------------------

// URI takes params as a map
type ClientURI struct {
	remote string
	client *http.Client
}

func NewClientURI(remote string) *ClientURI {
	return &ClientURI{
		remote: remote,
		client: &http.Client{Transport: socketTransport(remote)},
	}
}

func (c *ClientURI) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	return c.call(method, params, result)
}

func (c *ClientURI) call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	values, err := argsToURLValues(params)
	if err != nil {
		return nil, err
	}
	log.Info(Fmt("URI request to %v (%v): %v", c.remote, method, values))
	resp, err := c.client.PostForm(dummyDomain+method, values)
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
