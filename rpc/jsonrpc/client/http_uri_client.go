package client

import (
	"fmt"
	"io/ioutil"
	"net/http"

	types "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const (
	// URIClientRequestID in a request ID used by URIClient
	URIClientRequestID = types.JSONRPCIntID(-1)
)

// URIClient is a JSON-RPC client, which sends POST form HTTP requests to the
// remote server.
//
// URIClient is safe for concurrent use by multiple goroutines.
type URIClient struct {
	address string
	client  *http.Client
}

var _ HTTPClient = (*URIClient)(nil)

// NewURI returns a new client.
// An error is returned on invalid remote.
// The function panics when remote is nil.
func NewURI(remote string) (*URIClient, error) {
	parsedURL, err := newParsedURL(remote)
	if err != nil {
		return nil, err
	}

	httpClient, err := DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}

	parsedURL.SetDefaultSchemeHTTP()

	uriClient := &URIClient{
		address: parsedURL.GetTrimmedURL(),
		client:  httpClient,
	}

	return uriClient, nil
}

// Call issues a POST form HTTP request.
func (c *URIClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	values, err := argsToURLValues(params)
	if err != nil {
		return nil, fmt.Errorf("failed to encode params: %w", err)
	}

	resp, err := c.client.PostForm(c.address+"/"+method, values)
	if err != nil {
		return nil, fmt.Errorf("post form failed: %w", err)
	}
	defer resp.Body.Close()

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return unmarshalResponseBytes(responseBytes, URIClientRequestID, result)
}
