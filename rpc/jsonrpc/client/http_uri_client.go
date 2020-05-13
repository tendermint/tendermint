package client

import (
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	types "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const (
	// URIClientRequestID in a request ID used by URIClient
	URIClientRequestID = types.JSONRPCIntID(-1)
)

// URIClient is a JSON-RPC client, which sends POST form HTTP requests to the
// remote server.
//
// Request values are amino encoded. Response is expected to be amino encoded.
// New amino codec is used if no other codec was set using SetCodec.
//
// URIClient is safe for concurrent use by multiple goroutines.
type URIClient struct {
	address string
	client  *http.Client
	cdc     *amino.Codec
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
		cdc:     amino.NewCodec(),
	}

	return uriClient, nil
}

// Call issues a POST form HTTP request.
func (c *URIClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	values, err := argsToURLValues(c.cdc, params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode params")
	}

	resp, err := c.client.PostForm(c.address+"/"+method, values)
	if err != nil {
		return nil, errors.Wrap(err, "PostForm failed")
	}
	defer resp.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	return unmarshalResponseBytes(c.cdc, responseBytes, URIClientRequestID, result)
}

func (c *URIClient) Codec() *amino.Codec       { return c.cdc }
func (c *URIClient) SetCodec(cdc *amino.Codec) { c.cdc = cdc }
