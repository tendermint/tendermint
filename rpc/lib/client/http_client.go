package rpcclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/tendermint/go-amino"

	types "github.com/tendermint/tendermint/rpc/lib/types"
)

const (
	protoHTTP  = "http"
	protoHTTPS = "https"
	protoWSS   = "wss"
	protoWS    = "ws"
	protoTCP   = "tcp"
)

// HTTPClient is a common interface for JSONRPCClient and URIClient.
type HTTPClient interface {
	Call(method string, params map[string]interface{}, result interface{}) (interface{}, error)
	Codec() *amino.Codec
	SetCodec(*amino.Codec)
}

// TODO: Deprecate support for IP:PORT or /path/to/socket
func makeHTTPDialer(remoteAddr string) (string, string, func(string, string) (net.Conn, error)) {
	// protocol to use for http operations, to support both http and https
	clientProtocol := protoHTTP

	parts := strings.SplitN(remoteAddr, "://", 2)
	var protocol, address string
	if len(parts) == 1 {
		// default to tcp if nothing specified
		protocol, address = protoTCP, remoteAddr
	} else if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	} else {
		// return a invalid message
		msg := fmt.Sprintf("Invalid addr: %s", remoteAddr)
		return clientProtocol, msg, func(_ string, _ string) (net.Conn, error) {
			return nil, errors.New(msg)
		}
	}

	// accept http as an alias for tcp and set the client protocol
	switch protocol {
	case protoHTTP, protoHTTPS:
		clientProtocol = protocol
		protocol = protoTCP
	case protoWS, protoWSS:
		clientProtocol = protocol
	}

	// replace / with . for http requests (kvstore domain)
	trimmedAddress := strings.Replace(address, "/", ".", -1)
	return clientProtocol, trimmedAddress, func(proto, addr string) (net.Conn, error) {
		return net.Dial(protocol, address)
	}
}

// We overwrite the http.Client.Dial so we can do http over tcp or unix.
// remoteAddr should be fully featured (eg. with tcp:// or unix://)
func makeHTTPClient(remoteAddr string) (string, *http.Client) {
	protocol, address, dialer := makeHTTPDialer(remoteAddr)
	return protocol + "://" + address, &http.Client{
		Transport: &http.Transport{
			Dial: dialer,
		},
	}
}

//------------------------------------------------------------------------------------

// JSONRPCClient takes params as a slice
type JSONRPCClient struct {
	address string
	client  *http.Client
	cdc     *amino.Codec
}

// NewJSONRPCClient returns a JSONRPCClient pointed at the given address.
func NewJSONRPCClient(remote string) *JSONRPCClient {
	address, client := makeHTTPClient(remote)
	return &JSONRPCClient{
		address: address,
		client:  client,
		cdc:     amino.NewCodec(),
	}
}

func (c *JSONRPCClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	request, err := types.MapToRequest(c.cdc, "jsonrpc-client", method, params)
	if err != nil {
		return nil, err
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	// log.Info(string(requestBytes))
	requestBuf := bytes.NewBuffer(requestBytes)
	// log.Info(Fmt("RPC request to %v (%v): %v", c.remote, method, string(requestBytes)))
	httpResponse, err := c.client.Post(c.address, "text/json", requestBuf)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	// 	log.Info(Fmt("RPC response: %v", string(responseBytes)))
	return unmarshalResponseBytes(c.cdc, responseBytes, result)
}

func (c *JSONRPCClient) Codec() *amino.Codec {
	return c.cdc
}

func (c *JSONRPCClient) SetCodec(cdc *amino.Codec) {
	c.cdc = cdc
}

//-------------------------------------------------------------

// URI takes params as a map
type URIClient struct {
	address string
	client  *http.Client
	cdc     *amino.Codec
}

func NewURIClient(remote string) *URIClient {
	address, client := makeHTTPClient(remote)
	return &URIClient{
		address: address,
		client:  client,
		cdc:     amino.NewCodec(),
	}
}

func (c *URIClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	values, err := argsToURLValues(c.cdc, params)
	if err != nil {
		return nil, err
	}
	// log.Info(Fmt("URI request to %v (%v): %v", c.address, method, values))
	resp, err := c.client.PostForm(c.address+"/"+method, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return unmarshalResponseBytes(c.cdc, responseBytes, result)
}

func (c *URIClient) Codec() *amino.Codec {
	return c.cdc
}

func (c *URIClient) SetCodec(cdc *amino.Codec) {
	c.cdc = cdc
}

//------------------------------------------------

func unmarshalResponseBytes(cdc *amino.Codec, responseBytes []byte, result interface{}) (interface{}, error) {
	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	// log.Notice("response", "response", string(responseBytes))
	var err error
	response := &types.RPCResponse{}
	err = json.Unmarshal(responseBytes, response)
	if err != nil {
		return nil, errors.Errorf("Error unmarshalling rpc response: %v", err)
	}
	if response.Error != nil {
		return nil, errors.Errorf("Response error: %v", response.Error)
	}
	// Unmarshal the RawMessage into the result.
	err = cdc.UnmarshalJSON(response.Result, result)
	if err != nil {
		return nil, errors.Errorf("Error unmarshalling rpc response result: %v", err)
	}
	return result, nil
}

func argsToURLValues(cdc *amino.Codec, args map[string]interface{}) (url.Values, error) {
	values := make(url.Values)
	if len(args) == 0 {
		return values, nil
	}
	err := argsToJSON(cdc, args)
	if err != nil {
		return nil, err
	}
	for key, val := range args {
		values.Set(key, val.(string))
	}
	return values, nil
}

func argsToJSON(cdc *amino.Codec, args map[string]interface{}) error {
	for k, v := range args {
		rt := reflect.TypeOf(v)
		isByteSlice := rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.Uint8
		if isByteSlice {
			bytes := reflect.ValueOf(v).Bytes()
			args[k] = fmt.Sprintf("0x%X", bytes)
			continue
		}

		data, err := cdc.MarshalJSON(v)
		if err != nil {
			return err
		}
		args[k] = string(data)
	}
	return nil
}
