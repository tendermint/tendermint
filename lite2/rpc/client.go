package rpc

import (
	"bytes"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto/merkle"
	lite "github.com/tendermint/tendermint/lite2"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

// Client is an RPC client, which uses lite#Client to verify data (if it can be
// proved!).
type Client struct {
	next rpcclient.Client
	lc   *lite.Client
	prt  *merkle.ProofRuntime
}

var _ rpcclient.Client = (*Client)(nil)

// NewClient returns a new client.
func NewClient(next rpcclient.Client, lc *lite.Client) *Client {
	return &Client{
		next: next,
		lc:   lc,
		prt:  defaultProofRuntime(),
	}
}

func (c *Client) Status() (*ctypes.ResultStatus, error) {
	return c.next.Status()
}

func (c *Client) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return c.next.ABCIInfo()
}

func (c *Client) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *Client) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return c.next.ABCIQueryWithOptions(path, data, opts)
}

func (c *Client) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return c.next.BroadcastTxCommit(tx)
}

func (c *Client) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTxAsync(tx)
}

func (c *Client) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTxSync(tx)
}

func (c *Client) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	return c.next.UnconfirmedTxs(limit)
}

func (c *Client) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return c.next.NumUnconfirmedTxs()
}

func (c *Client) NetInfo() (*ctypes.ResultNetInfo, error) {
	return c.next.NetInfo()
}

func (c *Client) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	return c.next.DumpConsensusState()
}

func (c *Client) ConsensusState() (*ctypes.ResultConsensusState, error) {
	return c.next.ConsensusState()
}

func (c *Client) Health() (*ctypes.ResultHealth, error) {
	return c.next.Health()
}

// BlockchainInfo calls rpcclient#BlockchainInfo and then verifies every header
// returned.
func (c *Client) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	res, err := c.next.BlockchainInfo(minHeight, maxHeight)
	if err != nil {
		return nil, err
	}

	// Validate res.
	for _, meta := range res.BlockMetas {
		if meta == nil {
			return nil, errors.New("nil BlockMeta")
		}
		if err := meta.ValidateBasic(); err != nil {
			return nil, errors.Wrap(err, "invalid BlockMeta")
		}
	}

	// Update the light client if we're behind.
	if len(res.BlockMetas) > 0 {
		lastHeight := res.BlockMetas[len(res.BlockMetas)-1].Height
		if c.lc.LastTrustedHeight() < lastHeight {
			if err := c.lc.VerifyHeaderAtHeight(lastHeight, time.Now()); err != nil {
				return errors.Wrapf(err, "VerifyHeaderAtHeight(%d)", lastHeight)
			}
		}
	}

	// Verify each of the BlockMetas.
	for _, meta := range res.BlockMetas {
		h, err := c.lc.TrustedHeader(meta.Header.Height)
		if err != nil {
			return nil, errors.Wrapf(err, "TrustedHeader(%d)", meta.Header.Height)
		}
		if !bytes.Equal(meta.Header.Hash, h.Hash) {
			return nil, errors.Errorf("BlockMeta#Header %X does not match with trusted header %X",
				meta.Header.Hash, h.Hash)
		}
	}

	return res, nil
}

func (c *Client) Genesis() (*ctypes.ResultGenesis, error) {
	return c.next.Genesis()
}

func (c *Client) Block(height *int64) (*ctypes.ResultBlock, error) {
	return c.next.Block(height)
}

func (c *Client) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	return c.next.BlockResults(height)
}

func (c *Client) Commit(height *int64) (*ctypes.ResultCommit, error) {
	return c.next.Commit(height)
}

// Tx calls rpcclient#Tx method and then verifies the proof if such was
// requested.
func (c *Client) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	res, err := c.next.Tx(hash, prove)
	if err != nil || !prove {
		return res, err
	}

	// Validate res.
	if res.Height <= 0 || res.Proof == nil {
		return nil, errors.Errorf("invalid ResultTx: %v", res)
	}

	// Update the light client if we're behind.
	if c.lc.LastTrustedHeight() < res.Height {
		if err := c.lc.VerifyHeaderAtHeight(res.Height, time.Now()); err != nil {
			return errors.Wrapf(err, "VerifyHeaderAtHeight(%d)", res.Height)
		}
	}

	// Validate the proof.
	h, err := c.lc.TrustedHeader(res.Height)
	if err != nil {
		return res, errors.Wrapf(err, "TrustedHeader(%d)", res.Height)
	}
	return res, res.Proof.Validate(h.DataHash)
}

func (c *Client) TxSearch(query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	return c.next.TxSearch(query, prove, page, perPage)
}

func (c *Client) Validators(height *int64) (*ctypes.ResultValidators, error) {
	return c.next.Validators(height)
}

func (c *Client) BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return c.next.BroadcastEvidence(ev)
}
