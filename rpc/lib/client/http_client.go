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
	"sync"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	cmn "github.com/tendermint/tendermint/libs/common"
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

// protocol - client's protocol (for example, "http", "https", "wss", "ws", "tcp")
// trimmedS - rest of the address (for example, "192.0.2.1:25", "[2001:db8::1]:80") with "/" replaced with "."
func toClientAddrAndParse(remoteAddr string) (network string, trimmedS string, err error) {
	protocol, address, err := parseRemoteAddr(remoteAddr)
	if err != nil {
		return "", "", err
	}

	// protocol to use for http operations, to support both http and https
	var clientProtocol string
	// default to http for unknown protocols (ex. tcp)
	switch protocol {
	case protoHTTP, protoHTTPS, protoWS, protoWSS:
		clientProtocol = protocol
	default:
		clientProtocol = protoHTTP
	}

	// replace / with . for http requests (kvstore domain)
	trimmedAddress := strings.Replace(address, "/", ".", -1)
	return clientProtocol, trimmedAddress, nil
}

func toClientAddress(remoteAddr string) (string, error) {
	clientProtocol, trimmedAddress, err := toClientAddrAndParse(remoteAddr)
	if err != nil {
		return "", err
	}
	return clientProtocol + "://" + trimmedAddress, nil
}

// network - name of the network (for example, "tcp", "unix")
// s - rest of the address (for example, "192.0.2.1:25", "[2001:db8::1]:80")
// TODO: Deprecate support for IP:PORT or /path/to/socket
func parseRemoteAddr(remoteAddr string) (network string, s string, err error) {
	parts := strings.SplitN(remoteAddr, "://", 2)
	var protocol, address string
	switch {
	case len(parts) == 1:
		// default to tcp if nothing specified
		protocol, address = protoTCP, remoteAddr
	case len(parts) == 2:
		protocol, address = parts[0], parts[1]
	default:
		return "", "", fmt.Errorf("invalid addr: %s", remoteAddr)
	}

	// accept http(s) as an alias for tcp
	switch protocol {
	case protoHTTP, protoHTTPS:
		protocol = protoTCP
	}

	return protocol, address, nil
}

func makeErrorDialer(err error) func(string, string) (net.Conn, error) {
	return func(_ string, _ string) (net.Conn, error) {
		return nil, err
	}
}

func makeHTTPDialer(remoteAddr string) func(string, string) (net.Conn, error) {
	protocol, address, err := parseRemoteAddr(remoteAddr)
	if err != nil {
		return makeErrorDialer(err)
	}

	return func(proto, addr string) (net.Conn, error) {
		return net.Dial(protocol, address)
	}
}

// DefaultHTTPClient is used to create an http client with some default parameters.
// We overwrite the http.Client.Dial so we can do http over tcp or unix.
// remoteAddr should be fully featured (eg. with tcp:// or unix://)
func DefaultHTTPClient(remoteAddr string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			// Set to true to prevent GZIP-bomb DoS attacks
			DisableCompression: true,
			Dial:               makeHTTPDialer(remoteAddr),
		},
	}
}

//------------------------------------------------------------------------------------

// jsonRPCBufferedRequest encapsulates a single buffered request, as well as its
// anticipated response structure.
type jsonRPCBufferedRequest struct {
	request types.RPCRequest
	result  interface{} // The result will be deserialized into this object.
}

// JSONRPCRequestBatch allows us to buffer multiple request/response structures
// into a single batch request. Note that this batch acts like a FIFO queue, and
// is thread-safe.
type JSONRPCRequestBatch struct {
	client *JSONRPCClient

	mtx      sync.Mutex
	requests []*jsonRPCBufferedRequest
}

// JSONRPCClient takes params as a slice
type JSONRPCClient struct {
	address string
	client  *http.Client
	id      types.JSONRPCStringID
	cdc     *amino.Codec
}

// JSONRPCCaller implementers can facilitate calling the JSON RPC endpoint.
type JSONRPCCaller interface {
	Call(method string, params map[string]interface{}, result interface{}) (interface{}, error)
}

// Both JSONRPCClient and JSONRPCRequestBatch can facilitate calls to the JSON
// RPC endpoint.
var _ JSONRPCCaller = (*JSONRPCClient)(nil)
var _ JSONRPCCaller = (*JSONRPCRequestBatch)(nil)

// NewJSONRPCClient returns a JSONRPCClient pointed at the given address.
func NewJSONRPCClient(remote string) *JSONRPCClient {
	return NewJSONRPCClientWithHTTPClient(remote, DefaultHTTPClient(remote))
}

// NewJSONRPCClientWithHTTPClient returns a JSONRPCClient pointed at the given address using a custom http client
// The function panics if the provided client is nil or remote is invalid.
func NewJSONRPCClientWithHTTPClient(remote string, client *http.Client) *JSONRPCClient {
	if client == nil {
		panic("nil http.Client provided")
	}

	clientAddress, err := toClientAddress(remote)
	if err != nil {
		panic(fmt.Sprintf("invalid remote %s: %s", remote, err))
	}

	return &JSONRPCClient{
		address: clientAddress,
		client:  client,
		id:      types.JSONRPCStringID("jsonrpc-client-" + cmn.RandStr(8)),
		cdc:     amino.NewCodec(),
	}
}

// Call will send the request for the given method through to the RPC endpoint
// immediately, without buffering of requests.
func (c *JSONRPCClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	request, err := types.MapToRequest(c.cdc, c.id, method, params)
	if err != nil {
		return nil, err
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	requestBuf := bytes.NewBuffer(requestBytes)
	httpResponse, err := c.client.Post(c.address, "text/json", requestBuf)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	return unmarshalResponseBytes(c.cdc, responseBytes, c.id, result)
}

// NewRequestBatch starts a batch of requests for this client.
func (c *JSONRPCClient) NewRequestBatch() *JSONRPCRequestBatch {
	return &JSONRPCRequestBatch{
		requests: make([]*jsonRPCBufferedRequest, 0),
		client:   c,
	}
}

func (c *JSONRPCClient) sendBatch(requests []*jsonRPCBufferedRequest) ([]interface{}, error) {
	reqs := make([]types.RPCRequest, 0, len(requests))
	results := make([]interface{}, 0, len(requests))
	for _, req := range requests {
		reqs = append(reqs, req.request)
		results = append(results, req.result)
	}
	// serialize the array of requests into a single JSON object
	requestBytes, err := json.Marshal(reqs)
	if err != nil {
		return nil, err
	}
	httpResponse, err := c.client.Post(c.address, "text/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	return unmarshalResponseBytesArray(c.cdc, responseBytes, c.id, results)
}

func (c *JSONRPCClient) Codec() *amino.Codec {
	return c.cdc
}

func (c *JSONRPCClient) SetCodec(cdc *amino.Codec) {
	c.cdc = cdc
}

//-------------------------------------------------------------

// Count returns the number of enqueued requests waiting to be sent.
func (b *JSONRPCRequestBatch) Count() int {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return len(b.requests)
}

func (b *JSONRPCRequestBatch) enqueue(req *jsonRPCBufferedRequest) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.requests = append(b.requests, req)
}

// Clear empties out the request batch.
func (b *JSONRPCRequestBatch) Clear() int {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.clear()
}

func (b *JSONRPCRequestBatch) clear() int {
	count := len(b.requests)
	b.requests = make([]*jsonRPCBufferedRequest, 0)
	return count
}

// Send will attempt to send the current batch of enqueued requests, and then
// will clear out the requests once done. On success, this returns the
// deserialized list of results from each of the enqueued requests.
func (b *JSONRPCRequestBatch) Send() ([]interface{}, error) {
	b.mtx.Lock()
	defer func() {
		b.clear()
		b.mtx.Unlock()
	}()
	return b.client.sendBatch(b.requests)
}

// Call enqueues a request to call the given RPC method with the specified
// parameters, in the same way that the `JSONRPCClient.Call` function would.
func (b *JSONRPCRequestBatch) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	request, err := types.MapToRequest(b.client.cdc, b.client.id, method, params)
	if err != nil {
		return nil, err
	}
	b.enqueue(&jsonRPCBufferedRequest{request: request, result: result})
	return result, nil
}

//-------------------------------------------------------------

// URI takes params as a map
type URIClient struct {
	address string
	client  *http.Client
	cdc     *amino.Codec
}

// The function panics if the provided remote is invalid.
func NewURIClient(remote string) *URIClient {
	clientAddress, err := toClientAddress(remote)
	if err != nil {
		panic(fmt.Sprintf("invalid remote %s: %s", remote, err))
	}
	return &URIClient{
		address: clientAddress,
		client:  DefaultHTTPClient(remote),
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
	return unmarshalResponseBytes(c.cdc, responseBytes, "", result)
}

func (c *URIClient) Codec() *amino.Codec {
	return c.cdc
}

func (c *URIClient) SetCodec(cdc *amino.Codec) {
	c.cdc = cdc
}

//------------------------------------------------

func unmarshalResponseBytes(cdc *amino.Codec, responseBytes []byte, expectedID types.JSONRPCStringID, result interface{}) (interface{}, error) {
	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	// log.Notice("response", "response", string(responseBytes))
	var err error
	response := &types.RPCResponse{}
	err = json.Unmarshal(responseBytes, response)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling rpc response")
	}
	if response.Error != nil {
		return nil, errors.Wrap(response.Error, "response error")
	}
	// From the JSON-RPC 2.0 spec:
	//  id: It MUST be the same as the value of the id member in the Request Object.
	if err := validateResponseID(response, expectedID); err != nil {
		return nil, err
	}
	// Unmarshal the RawMessage into the result.
	err = cdc.UnmarshalJSON(response.Result, result)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling rpc response result")
	}
	return result, nil
}

func unmarshalResponseBytesArray(cdc *amino.Codec, responseBytes []byte, expectedID types.JSONRPCStringID, results []interface{}) ([]interface{}, error) {
	var (
		err       error
		responses []types.RPCResponse
	)
	err = json.Unmarshal(responseBytes, &responses)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling rpc response")
	}
	// No response error checking here as there may be a mixture of successful
	// and unsuccessful responses.

	if len(results) != len(responses) {
		return nil, errors.Errorf("expected %d result objects into which to inject responses, but got %d", len(responses), len(results))
	}

	for i, response := range responses {
		response := response
		// From the JSON-RPC 2.0 spec:
		//  id: It MUST be the same as the value of the id member in the Request Object.
		if err := validateResponseID(&response, expectedID); err != nil {
			return nil, errors.Wrapf(err, "failed to validate response ID in response %d", i)
		}
		if err := cdc.UnmarshalJSON(responses[i].Result, results[i]); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling rpc response result")
		}
	}
	return results, nil
}

func validateResponseID(res *types.RPCResponse, expectedID types.JSONRPCStringID) error {
	// we only validate a response ID if the expected ID is non-empty
	if len(expectedID) == 0 {
		return nil
	}
	if res.ID == nil {
		return errors.Errorf("missing ID in response")
	}
	id, ok := res.ID.(types.JSONRPCStringID)
	if !ok {
		return errors.Errorf("expected ID string in response but got: %v", id)
	}
	if expectedID != id {
		return errors.Errorf("response ID (%s) does not match request ID (%s)", id, expectedID)
	}
	return nil
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
