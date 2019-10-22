package rpc

import (
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tendermint/libs/common"
	lite "github.com/tendermint/tendermint/lite2"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Client is an RPC client, which uses lite#Client to verify data (if it can be
// proved!).
type Client struct {
	cmn.BaseService

	next rpcclient.Client
	lc   *lite.Client
	prt  *merkle.ProofRuntime
}

var _ rpcclient.Client = (*Client)(nil)

// NewClient returns a new client.
func NewClient(next rpcclient.Client, lc *lite.Client) *Client {
	c := &Client{
		next: next,
		lc:   lc,
		prt:  defaultProofRuntime(),
	}
	c.BaseService = *cmn.NewBaseService(nil, "Client", c)
	return c
}

func (c *Client) OnStart() error {
	if !c.next.IsRunning() {
		return c.next.Start()
	}
	return nil
}

func (c *Client) OnStop() {
	if c.next.IsRunning() {
		c.next.Stop()
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

func (c *Client) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts rpcclient.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
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
		lastHeight := res.BlockMetas[len(res.BlockMetas)-1].Header.Height
		if err := c.updateLiteClientIfNeededTo(lastHeight); err != nil {
			return nil, err
		}
	}

	// Verify each of the BlockMetas.
	for _, meta := range res.BlockMetas {
		h, err := c.lc.TrustedHeader(meta.Header.Height)
		if err != nil {
			return nil, errors.Wrapf(err, "TrustedHeader(%d)", meta.Header.Height)
		}
		if bmH, tH := meta.Header.Hash(), h.Hash(); !bytes.Equal(bmH, tH) {
			return nil, errors.Errorf("BlockMeta#Header %X does not match with trusted header %X",
				bmH, tH)
		}
	}

	return res, nil
}

func (c *Client) Genesis() (*ctypes.ResultGenesis, error) {
	return c.next.Genesis()
}

// Block calls rpcclient#Block and then verifies the result.
func (c *Client) Block(height *int64) (*ctypes.ResultBlock, error) {
	res, err := c.next.Block(height)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if err := res.BlockMeta.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := res.Block.ValidateBasic(); err != nil {
		return nil, err
	}
	if bmH, bH := res.BlockMeta.Header.Hash(), res.Block.Hash(); !bytes.Equal(bmH, bH) {
		return nil, errors.Errorf("BlockMeta#Header %X does not match with Block %X",
			bmH, bH)
	}

	// Update the light client if we're behind.
	if err := c.updateLiteClientIfNeededTo(res.Block.Height); err != nil {
		return nil, err
	}

	// Verify block.
	h, err := c.lc.TrustedHeader(res.Block.Height)
	if err != nil {
		return nil, errors.Wrapf(err, "TrustedHeader(%d)", res.Block.Height)
	}
	if bH, tH := res.Block.Hash(), h.Hash(); !bytes.Equal(bH, tH) {
		return nil, errors.Errorf("Block#Header %X does not match with trusted header %X",
			bH, tH)
	}

	return res, nil
}

func (c *Client) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	return c.next.BlockResults(height)
}

func (c *Client) Commit(height *int64) (*ctypes.ResultCommit, error) {
	res, err := c.next.Commit(height)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if err := res.SignedHeader.ValidateBasic(c.lc.ChainID()); err != nil {
		return nil, err
	}

	// Update the light client if we're behind.
	if err := c.updateLiteClientIfNeededTo(res.Height); err != nil {
		return nil, err
	}

	// Verify commit.
	h, err := c.lc.TrustedHeader(res.Height)
	if err != nil {
		return nil, errors.Wrapf(err, "TrustedHeader(%d)", res.Height)
	}
	if rH, tH := res.Hash(), h.Hash(); !bytes.Equal(rH, tH) {
		return nil, errors.Errorf("header %X does not match with trusted header %X",
			rH, tH)
	}

	return res, nil
}

// Tx calls rpcclient#Tx method and then verifies the proof if such was
// requested.
func (c *Client) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	res, err := c.next.Tx(hash, prove)
	if err != nil || !prove {
		return res, err
	}

	// Validate res.
	if res.Height <= 0 {
		return nil, errors.Errorf("invalid ResultTx: %v", res)
	}

	// Update the light client if we're behind.
	if err := c.updateLiteClientIfNeededTo(res.Height); err != nil {
		return nil, err
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

func (c *Client) Subscribe(ctx context.Context, subscriber, query string,
	outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	return c.next.Subscribe(ctx, subscriber, query, outCapacity...)
}

func (c *Client) Unsubscribe(ctx context.Context, subscriber, query string) error {
	return c.next.Unsubscribe(ctx, subscriber, query)
}

func (c *Client) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.next.UnsubscribeAll(ctx, subscriber)
}

func (c *Client) updateLiteClientIfNeededTo(height int64) error {
	lastTrustedHeight, err := c.lc.LastTrustedHeight()
	if err != nil {
		return errors.Wrap(err, "LastTrustedHeight")
	}
	if lastTrustedHeight < height {
		if err := c.lc.VerifyHeaderAtHeight(height, time.Now()); err != nil {
			return errors.Wrapf(err, "VerifyHeaderAtHeight(%d)", height)
		}
	}
	return nil
}
