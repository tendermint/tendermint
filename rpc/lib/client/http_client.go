package rpcclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	types "github.com/tendermint/tendermint/rpc/lib/types"
)

const (
	protoHTTP  = "http"
	protoHTTPS = "https"
	protoWSS   = "wss"
	protoWS    = "ws"
	protoTCP   = "tcp"
)

//-------------------------------------------------------------

// HTTPClient is a common interface for JSON-RPC clients.
type HTTPClient interface {
	// Call calls the given method with the params and returns a result.
	Call(method string, params map[string]interface{}, result interface{}) (interface{}, error)
	// Codec returns an amino codec used.
	Codec() *amino.Codec
	// SetCodec sets an amino codec.
	SetCodec(*amino.Codec)
}

// JSONRPCCaller implementers can facilitate calling the JSON-RPC endpoint.
type JSONRPCCaller interface {
	Call(method string, params map[string]interface{}, result interface{}) (interface{}, error)
}

//-------------------------------------------------------------

// JSONRPCClient is a JSON-RPC client, which sends POST HTTP requests to the
// remote server.
//
// Request values are amino encoded. Response is expected to be amino encoded.
// New amino codec is used if no other codec was set using SetCodec.
type JSONRPCClient struct {
	address string
	client  *http.Client
	cdc     *amino.Codec

	nextReqID uint
}

var _ HTTPClient = (*JSONRPCClient)(nil)

// Both JSONRPCClient and JSONRPCRequestBatch can facilitate calls to the JSON
// RPC endpoint.
var _ JSONRPCCaller = (*JSONRPCClient)(nil)
var _ JSONRPCCaller = (*JSONRPCRequestBatch)(nil)

// NewJSONRPCClient returns a JSONRPCClient pointed at the given address.
func NewJSONRPCClient(remote string) *JSONRPCClient {
	address, client := makeHTTPClient(remote)
	return &JSONRPCClient{
		address: address,
		client:  client,
		cdc:     amino.NewCodec(),
	}
}

// Call issues a POST HTTP request. Requests are JSON encoded. Content-Type:
// text/json.
func (c *JSONRPCClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	id := c.nextRequestID()

	request, err := types.MapToRequest(c.cdc, id, method, params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode params")
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	requestBuf := bytes.NewBuffer(requestBytes)
	httpResponse, err := c.client.Post(c.address, "text/json", requestBuf)
	if err != nil {
		return nil, errors.Wrap(err, "Post failed")
	}
	defer httpResponse.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	return unmarshalResponseBytes(c.cdc, responseBytes, id, result)
}

func (c *JSONRPCClient) Codec() *amino.Codec       { return c.cdc }
func (c *JSONRPCClient) SetCodec(cdc *amino.Codec) { c.cdc = cdc }

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

	// collect ids to check them in unmarshalResponseBytesArray
	ids := make([]types.JSONRPCIntID, len(requests))
	for i, req := range requests {
		ids[i] = req.request.ID.(types.JSONRPCIntID)
	}

	return unmarshalResponseBytesArray(c.cdc, responseBytes, ids, results)
}

func (c *JSONRPCClient) nextRequestID() types.JSONRPCIntID {
	id := c.nextReqID
	c.nextReqID++
	return types.JSONRPCIntID(id)
}

//-------------------------------------------------------------

// jsonRPCBufferedRequest encapsulates a single buffered request, as well as
// its anticipated response structure.
type jsonRPCBufferedRequest struct {
	request types.RPCRequest
	result  interface{} // The result will be deserialized into this object.
}

// JSONRPCRequestBatch allows us to buffer multiple request/response structures
// into a single batch request. Note that this batch acts like a FIFO queue,
// and is thread-safe.
type JSONRPCRequestBatch struct {
	client *JSONRPCClient

	mtx      sync.Mutex
	requests []*jsonRPCBufferedRequest
}

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
	id := b.client.nextRequestID()

	request, err := types.MapToRequest(b.client.cdc, id, method, params)
	if err != nil {
		return nil, err
	}

	b.enqueue(&jsonRPCBufferedRequest{request: request, result: result})

	return result, nil
}

//-------------------------------------------------------------

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
			// Set to true to prevent GZIP-bomb DoS attacks
			DisableCompression: true,
			Dial:               dialer,
		},
	}
}
