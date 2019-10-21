package http

import (
	lite "github.com/tendermint/tendermint/lite2"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

// A Client is a HTTP client, which uses lite#Client to verify data.
type Client struct {
	lite lite.Client
	http *rpcclient.HTTP
}

var _ rpcclient.Client = (*Client)(nil)

func (c *Client) Status() (*ctypes.ResultStatus, error) {
	return c.http.Status()
}

func (c *Client) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return c.http.ABCIInfo()
}

func (c *Client) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *Client) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return c.http.ABCIQueryWithOptions(path, data, opts)
}

func (c *Client) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return c.http.BroadcastTxCommit(tx)
}

func (c *Client) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.http.BroadcastTxAsync(tx)
}

func (c *Client) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.http.BroadcastTxSync(tx)
}

func (c *Client) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	return c.http.UnconfirmedTxs(limit)
}

func (c *Client) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return c.http.NumUnconfirmedTxs()
}

func (c *Client) NetInfo() (*ctypes.ResultNetInfo, error) {
	return c.http.NetInfo()
}

func (c *Client) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	return c.http.DumpConsensusState()
}

func (c *Client) ConsensusState() (*ctypes.ResultConsensusState, error) {
	return c.http.ConsensusState()
}

func (c *Client) Health() (*ctypes.ResultHealth, error) {
	return c.http.Health()
}

func (c *Client) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return c.http.BlockchainInfo(minHeight, maxHeight)
}

func (c *Client) Genesis() (*ctypes.ResultGenesis, error) {
	return c.http.Genesis()
}

func (c *Client) Block(height *int64) (*ctypes.ResultBlock, error) {
	return c.http.Block(height)
}

func (c *Client) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	return c.http.BlockResults(height)
}

func (c *Client) Commit(height *int64) (*ctypes.ResultCommit, error) {
	return c.http.Commit(height)
}

func (c *Client) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return c.http.Tx(hash, prove)
}

func (c *Client) TxSearch(query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	return c.http.TxSearch(query, prove, page, perPage)
}

func (c *Client) Validators(height *int64) (*ctypes.ResultValidators, error) {
	return c.http.Validators(height)
}

func (c *Client) BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return c.http.BroadcastEvidence(ev)
}
