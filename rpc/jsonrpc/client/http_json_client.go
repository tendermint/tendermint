package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const (
	protoHTTP  = "http"
	protoHTTPS = "https"
	protoWSS   = "wss"
	protoWS    = "ws"
	protoTCP   = "tcp"
	protoUNIX  = "unix"
)

//-------------------------------------------------------------

// Parsed URL structure
type parsedURL struct {
	url.URL

	isUnixSocket bool
}

// Parse URL and set defaults
func newParsedURL(remoteAddr string) (*parsedURL, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return nil, err
	}

	// default to tcp if nothing specified
	if u.Scheme == "" {
		u.Scheme = protoTCP
	}

	pu := &parsedURL{
		URL:          *u,
		isUnixSocket: false,
	}

	if u.Scheme == protoUNIX {
		pu.isUnixSocket = true
	}

	return pu, nil
}

// Change protocol to HTTP for unknown protocols and TCP protocol - useful for RPC connections
func (u *parsedURL) SetDefaultSchemeHTTP() {
	// protocol to use for http operations, to support both http and https
	switch u.Scheme {
	case protoHTTP, protoHTTPS, protoWS, protoWSS:
		// known protocols not changed
	default:
		// default to http for unknown protocols (ex. tcp)
		u.Scheme = protoHTTP
	}
}

// Get full address without the protocol - useful for Dialer connections
func (u parsedURL) GetHostWithPath() string {
	// Remove protocol, userinfo and # fragment, assume opaque is empty
	return u.Host + u.EscapedPath()
}

// Get a trimmed address - useful for WS connections
func (u parsedURL) GetTrimmedHostWithPath() string {
	// if it's not an unix socket we return the normal URL
	if !u.isUnixSocket {
		return u.GetHostWithPath()
	}
	// if it's a unix socket we replace the host slashes with a period
	// this is because otherwise the http.Client would think that the
	// domain is invalid.
	return strings.ReplaceAll(u.GetHostWithPath(), "/", ".")
}

// GetDialAddress returns the endpoint to dial for the parsed URL
func (u parsedURL) GetDialAddress() string {
	// if it's not a unix socket we return the host, example: localhost:443
	if !u.isUnixSocket {
		return u.Host
	}
	// otherwise we return the path of the unix socket, ex /tmp/socket
	return u.GetHostWithPath()
}

// Get a trimmed address with protocol - useful as address in RPC connections
func (u parsedURL) GetTrimmedURL() string {
	return u.Scheme + "://" + u.GetTrimmedHostWithPath()
}

//-------------------------------------------------------------

// A Caller handles the round trip of a single JSON-RPC request.  The
// implementation is responsible for assigning request IDs, marshaling
// parameters, and unmarshaling results.
type Caller interface {
	// Call sends a new request for method to the server with the given
	// parameters. If params == nil, the request has empty parameters.
	// If result == nil, any result value must be discarded without error.
	// Otherwise the concrete value of result must be a pointer.
	Call(ctx context.Context, method string, params, result interface{}) error
}

//-------------------------------------------------------------

// Client is a JSON-RPC client, which sends POST HTTP requests to the
// remote server.
//
// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	address  string
	username string
	password string

	client *http.Client

	mtx       sync.Mutex
	nextReqID int
}

// Both Client and RequestBatch can facilitate calls to the JSON
// RPC endpoint.
var _ Caller = (*Client)(nil)
var _ Caller = (*RequestBatch)(nil)

// New returns a Client pointed at the given address.
// An error is returned on invalid remote. The function panics when remote is nil.
func New(remote string) (*Client, error) {
	httpClient, err := DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}
	return NewWithHTTPClient(remote, httpClient)
}

// NewWithHTTPClient returns a Client pointed at the given address using a
// custom HTTP client. It reports an error if c == nil or if remote is not a
// valid URL.
func NewWithHTTPClient(remote string, c *http.Client) (*Client, error) {
	if c == nil {
		return nil, errors.New("nil client")
	}

	parsedURL, err := newParsedURL(remote)
	if err != nil {
		return nil, fmt.Errorf("invalid remote %s: %s", remote, err)
	}

	parsedURL.SetDefaultSchemeHTTP()

	address := parsedURL.GetTrimmedURL()
	username := parsedURL.User.Username()
	password, _ := parsedURL.User.Password()

	rpcClient := &Client{
		address:  address,
		username: username,
		password: password,
		client:   c,
	}

	return rpcClient, nil
}

// Call issues a POST HTTP request. Requests are JSON encoded. Content-Type:
// application/json.
func (c *Client) Call(ctx context.Context, method string, params, result interface{}) error {
	id := c.nextRequestID()

	request := rpctypes.NewRequest(id)
	if err := request.SetMethodAndParams(method, params); err != nil {
		return fmt.Errorf("failed to encode params: %w", err)
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	requestBuf := bytes.NewBuffer(requestBytes)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.address, requestBuf)
	if err != nil {
		return fmt.Errorf("request setup failed: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	if c.username != "" || c.password != "" {
		httpRequest.SetBasicAuth(c.username, c.password)
	}

	httpResponse, err := c.client.Do(httpRequest)
	if err != nil {
		return err
	}

	responseBytes, err := io.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	return unmarshalResponseBytes(responseBytes, request.ID(), result)
}

// NewRequestBatch starts a batch of requests for this client.
func (c *Client) NewRequestBatch() *RequestBatch {
	return &RequestBatch{
		requests: make([]*jsonRPCBufferedRequest, 0),
		client:   c,
	}
}

func (c *Client) sendBatch(ctx context.Context, requests []*jsonRPCBufferedRequest) ([]interface{}, error) {
	reqs := make([]rpctypes.RPCRequest, 0, len(requests))
	results := make([]interface{}, 0, len(requests))
	for _, req := range requests {
		reqs = append(reqs, req.request)
		results = append(results, req.result)
	}

	// serialize the array of requests into a single JSON object
	requestBytes, err := json.Marshal(reqs)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.address, bytes.NewBuffer(requestBytes))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	if c.username != "" || c.password != "" {
		httpRequest.SetBasicAuth(c.username, c.password)
	}

	httpResponse, err := c.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}

	responseBytes, err := io.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// collect ids to check responses IDs in unmarshalResponseBytesArray
	ids := make([]string, len(requests))
	for i, req := range requests {
		ids[i] = req.request.ID()
	}

	if err := unmarshalResponseBytesArray(responseBytes, ids, results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) nextRequestID() int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	id := c.nextReqID
	c.nextReqID++
	return id
}

//------------------------------------------------------------------------------------

// jsonRPCBufferedRequest encapsulates a single buffered request, as well as its
// anticipated response structure.
type jsonRPCBufferedRequest struct {
	request rpctypes.RPCRequest
	result  interface{} // The result will be deserialized into this object.
}

// RequestBatch allows us to buffer multiple request/response structures
// into a single batch request. Note that this batch acts like a FIFO queue, and
// is thread-safe.
type RequestBatch struct {
	client *Client

	mtx      sync.Mutex
	requests []*jsonRPCBufferedRequest
}

// Count returns the number of enqueued requests waiting to be sent.
func (b *RequestBatch) Count() int {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return len(b.requests)
}

func (b *RequestBatch) enqueue(req *jsonRPCBufferedRequest) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.requests = append(b.requests, req)
}

// Clear empties out the request batch.
func (b *RequestBatch) Clear() int {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.clear()
}

func (b *RequestBatch) clear() int {
	count := len(b.requests)
	b.requests = make([]*jsonRPCBufferedRequest, 0)
	return count
}

// Send will attempt to send the current batch of enqueued requests, and then
// will clear out the requests once done. On success, this returns the
// deserialized list of results from each of the enqueued requests.
func (b *RequestBatch) Send(ctx context.Context) ([]interface{}, error) {
	b.mtx.Lock()
	defer func() {
		b.clear()
		b.mtx.Unlock()
	}()
	return b.client.sendBatch(ctx, b.requests)
}

// Call enqueues a request to call the given RPC method with the specified
// parameters, in the same way that the `Client.Call` function would.
func (b *RequestBatch) Call(_ context.Context, method string, params, result interface{}) error {
	request := rpctypes.NewRequest(b.client.nextRequestID())
	if err := request.SetMethodAndParams(method, params); err != nil {
		return err
	}
	b.enqueue(&jsonRPCBufferedRequest{request: request, result: result})
	return nil
}

//-------------------------------------------------------------

func makeHTTPDialer(remoteAddr string) (func(string, string) (net.Conn, error), error) {
	u, err := newParsedURL(remoteAddr)
	if err != nil {
		return nil, err
	}

	protocol := u.Scheme
	padding := u.Scheme

	// accept http(s) as an alias for tcp
	switch protocol {
	case protoHTTP, protoHTTPS:
		protocol = protoTCP
	}

	dialFn := func(proto, addr string) (net.Conn, error) {
		var timeout = 10 * time.Second
		if !u.isUnixSocket && strings.LastIndex(u.Host, ":") == -1 {
			u.Host = fmt.Sprintf("%s:%s", u.Host, padding)
			return net.DialTimeout(protocol, u.GetDialAddress(), timeout)
		}

		return net.DialTimeout(protocol, u.GetDialAddress(), timeout)
	}

	return dialFn, nil
}

// DefaultHTTPClient is used to create an http client with some default parameters.
// We overwrite the http.Client.Dial so we can do http over tcp or unix.
// remoteAddr should be fully featured (eg. with tcp:// or unix://).
// An error will be returned in case of invalid remoteAddr.
func DefaultHTTPClient(remoteAddr string) (*http.Client, error) {
	dialFn, err := makeHTTPDialer(remoteAddr)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			// Set to true to prevent GZIP-bomb DoS attacks
			DisableCompression: true,
			Dial:               dialFn,
		},
	}

	return client, nil
}
