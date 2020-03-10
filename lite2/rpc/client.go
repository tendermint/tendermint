package rpc

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	service "github.com/tendermint/tendermint/libs/service"
	lite "github.com/tendermint/tendermint/lite2"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// Client is an RPC client, which uses lite#Client to verify data (if it can be
// proved!).
type Client struct {
	service.BaseService

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
	c.BaseService = *service.NewBaseService(nil, "Client", c)
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

func (c *Client) ABCIQuery(path string, data tmbytes.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, rpcclient.DefaultABCIQueryOptions)
}

// GetWithProofOptions is useful if you want full access to the ABCIQueryOptions.
// XXX Usage of path?  It's not used, and sometimes it's /, sometimes /key, sometimes /store.
func (c *Client) ABCIQueryWithOptions(path string, data tmbytes.HexBytes,
	opts rpcclient.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {

	res, err := c.next.ABCIQueryWithOptions(path, data, opts)
	if err != nil {
		return nil, err
	}
	resp := res.Response

	// Validate the response.
	if resp.IsErr() {
		return nil, errors.Errorf("err response code: %v", resp.Code)
	}
	if len(resp.Key) == 0 || resp.Proof == nil {
		return nil, errors.New("empty tree")
	}
	if resp.Height <= 0 {
		return nil, errors.New("negative or zero height")
	}

	// Update the light client if we're behind.
	// NOTE: AppHash for height H is in header H+1.
	h, err := c.updateLiteClientIfNeededTo(resp.Height + 1)
	if err != nil {
		return nil, err
	}

	// Validate the value proof against the trusted header.
	if resp.Value != nil {
		// Value exists
		// XXX How do we encode the key into a string...
		storeName, err := parseQueryStorePath(path)
		if err != nil {
			return nil, err
		}
		kp := merkle.KeyPath{}
		kp = kp.AppendKey([]byte(storeName), merkle.KeyEncodingURL)
		kp = kp.AppendKey(resp.Key, merkle.KeyEncodingURL)
		err = c.prt.VerifyValue(resp.Proof, h.AppHash, kp.String(), resp.Value)
		if err != nil {
			return nil, errors.Wrap(err, "verify value proof")
		}
		return &ctypes.ResultABCIQuery{Response: resp}, nil
	}

	// OR validate the ansence proof against the trusted header.
	// XXX How do we encode the key into a string...
	err = c.prt.VerifyAbsence(resp.Proof, h.AppHash, string(resp.Key))
	if err != nil {
		return nil, errors.Wrap(err, "verify absence proof")
	}
	return &ctypes.ResultABCIQuery{Response: resp}, nil
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

func (c *Client) ConsensusParams(height *int64) (*ctypes.ResultConsensusParams, error) {
	return c.next.ConsensusParams(height)
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
		if _, err := c.updateLiteClientIfNeededTo(lastHeight); err != nil {
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
	if err := res.BlockID.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := res.Block.ValidateBasic(); err != nil {
		return nil, err
	}
	if bmH, bH := res.BlockID.Hash, res.Block.Hash(); !bytes.Equal(bmH, bH) {
		return nil, errors.Errorf("BlockID %X does not match with Block %X",
			bmH, bH)
	}

	// Update the light client if we're behind.
	h, err := c.updateLiteClientIfNeededTo(res.Block.Height)
	if err != nil {
		return nil, err
	}

	// Verify block.
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
	h, err := c.updateLiteClientIfNeededTo(res.Height)
	if err != nil {
		return nil, err
	}

	// Verify commit.
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
	h, err := c.updateLiteClientIfNeededTo(res.Height)
	if err != nil {
		return nil, err
	}

	// Validate the proof.
	return res, res.Proof.Validate(h.DataHash)
}

func (c *Client) TxSearch(query string, prove bool, page, perPage int, orderBy string) (
	*ctypes.ResultTxSearch, error) {
	return c.next.TxSearch(query, prove, page, perPage, orderBy)
}

func (c *Client) Validators(height *int64, page, perPage int) (*ctypes.ResultValidators, error) {
	return c.next.Validators(height, page, perPage)
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

func (c *Client) updateLiteClientIfNeededTo(height int64) (*types.SignedHeader, error) {
	h, err := c.lc.VerifyHeaderAtHeight(height, time.Now())
	return h, errors.Wrapf(err, "failed to update light client to %d", height)
}

func (c *Client) RegisterOpDecoder(typ string, dec merkle.OpDecoder) {
	c.prt.RegisterOpDecoder(typ, dec)
}

// SubscribeWS subscribes for events using the given query and remote address as
// a subscriber, but does not verify responses (UNSAFE)!
// TODO: verify data
func (c *Client) SubscribeWS(ctx *rpctypes.Context, query string) (*ctypes.ResultSubscribe, error) {
	out, err := c.next.Subscribe(context.Background(), ctx.RemoteAddr(), query)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case resultEvent := <-out:
				// We should have a switch here that performs a validation
				// depending on the event's type.
				ctx.WSConn.TryWriteRPCResponse(
					rpctypes.NewRPCSuccessResponse(
						ctx.WSConn.Codec(),
						rpctypes.JSONRPCStringID(fmt.Sprintf("%v#event", ctx.JSONReq.ID)),
						resultEvent,
					))
			case <-c.Quit():
				return
			}
		}
	}()

	return &ctypes.ResultSubscribe{}, nil
}

// UnsubscribeWS calls original client's Unsubscribe using remote address as a
// subscriber.
func (c *Client) UnsubscribeWS(ctx *rpctypes.Context, query string) (*ctypes.ResultUnsubscribe, error) {
	err := c.next.Unsubscribe(context.Background(), ctx.RemoteAddr(), query)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAllWS calls original client's UnsubscribeAll using remote address
// as a subscriber.
func (c *Client) UnsubscribeAllWS(ctx *rpctypes.Context) (*ctypes.ResultUnsubscribe, error) {
	err := c.next.UnsubscribeAll(context.Background(), ctx.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsubscribe{}, nil
}

func parseQueryStorePath(path string) (storeName string, err error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("expected path to start with /")
	}

	paths := strings.SplitN(path[1:], "/", 3)
	switch {
	case len(paths) != 3:
		return "", errors.New("expected format like /store/<storeName>/key")
	case paths[0] != "store":
		return "", errors.New("expected format like /store/<storeName>/key")
	case paths[2] != "key":
		return "", errors.New("expected format like /store/<storeName>/key")
	}

	return paths[1], nil
}
