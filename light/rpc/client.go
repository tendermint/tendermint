package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/gogo/protobuf/proto"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	service "github.com/tendermint/tendermint/libs/service"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/types"
)

// KeyPathFunc builds a merkle path out of the given path and key.
type KeyPathFunc func(path string, key []byte) (merkle.KeyPath, error)

// LightClient is an interface that contains functionality needed by Client from the light client.
//go:generate ../../scripts/mockery_generate.sh LightClient
type LightClient interface {
	ChainID() string
	Update(ctx context.Context, now time.Time) (*types.LightBlock, error)
	VerifyLightBlockAtHeight(ctx context.Context, height int64, now time.Time) (*types.LightBlock, error)
	TrustedLightBlock(height int64) (*types.LightBlock, error)
}

var _ rpcclient.Client = (*Client)(nil)

// Client is an RPC client, which uses light#Client to verify data (if it can
// be proved). Note, merkle.DefaultProofRuntime is used to verify values
// returned by ABCI#Query.
type Client struct {
	service.BaseService

	next rpcclient.Client
	lc   LightClient

	// proof runtime used to verify values returned by ABCIQuery
	prt       *merkle.ProofRuntime
	keyPathFn KeyPathFunc

	quitCh chan struct{}
}

var _ rpcclient.Client = (*Client)(nil)

// Option allow you to tweak Client.
type Option func(*Client)

// KeyPathFn option can be used to set a function, which parses a given path
// and builds the merkle path for the prover. It must be provided if you want
// to call ABCIQuery or ABCIQueryWithOptions.
func KeyPathFn(fn KeyPathFunc) Option {
	return func(c *Client) {
		c.keyPathFn = fn
	}
}

// DefaultMerkleKeyPathFn creates a function used to generate merkle key paths
// from a path string and a key. This is the default used by the cosmos SDK.
// This merkle key paths are required when verifying /abci_query calls
func DefaultMerkleKeyPathFn() KeyPathFunc {
	// regexp for extracting store name from /abci_query path
	storeNameRegexp := regexp.MustCompile(`\/store\/(.+)\/key`)

	return func(path string, key []byte) (merkle.KeyPath, error) {
		matches := storeNameRegexp.FindStringSubmatch(path)
		if len(matches) != 2 {
			return nil, fmt.Errorf("can't find store name in %s using %s", path, storeNameRegexp)
		}
		storeName := matches[1]

		kp := merkle.KeyPath{}
		kp = kp.AppendKey([]byte(storeName), merkle.KeyEncodingURL)
		kp = kp.AppendKey(key, merkle.KeyEncodingURL)
		return kp, nil
	}
}

// NewClient returns a new client.
func NewClient(next rpcclient.Client, lc LightClient, opts ...Option) *Client {
	c := &Client{
		next:   next,
		lc:     lc,
		prt:    merkle.DefaultProofRuntime(),
		quitCh: make(chan struct{}),
	}
	c.BaseService = *service.NewBaseService(nil, "Client", c)
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) OnStart(ctx context.Context) error {
	if !c.next.IsRunning() {
		return c.next.Start(ctx)
	}

	go func() {
		defer close(c.quitCh)
		c.Wait()
	}()

	return nil
}

func (c *Client) OnStop() {
	if c.next.IsRunning() {
		if err := c.next.Stop(); err != nil {
			c.Logger.Error("Error stopping on next", "err", err)
		}
	}
}

func (c *Client) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	return c.next.Status(ctx)
}

func (c *Client) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	return c.next.ABCIInfo(ctx)
}

// ABCIQuery requests proof by default.
func (c *Client) ABCIQuery(ctx context.Context, path string, data tmbytes.HexBytes) (*coretypes.ResultABCIQuery, error) { //nolint:lll
	return c.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

// ABCIQueryWithOptions returns an error if opts.Prove is false.
func (c *Client) ABCIQueryWithOptions(ctx context.Context, path string, data tmbytes.HexBytes,
	opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {

	// always request the proof
	opts.Prove = true

	res, err := c.next.ABCIQueryWithOptions(ctx, path, data, opts)
	if err != nil {
		return nil, err
	}
	resp := res.Response

	// Validate the response.
	if resp.IsErr() {
		return nil, fmt.Errorf("err response code: %v", resp.Code)
	}
	if len(resp.Key) == 0 {
		return nil, errors.New("empty key")
	}
	if resp.ProofOps == nil || len(resp.ProofOps.Ops) == 0 {
		return nil, errors.New("no proof ops")
	}
	if resp.Height <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	// Update the light client if we're behind.
	// NOTE: AppHash for height H is in header H+1.
	nextHeight := resp.Height + 1
	l, err := c.updateLightClientIfNeededTo(ctx, &nextHeight)
	if err != nil {
		return nil, err
	}

	// Validate the value proof against the trusted header.
	if resp.Value != nil {
		// 1) build a Merkle key path from path and resp.Key
		if c.keyPathFn == nil {
			return nil, errors.New("please configure Client with KeyPathFn option")
		}

		kp, err := c.keyPathFn(path, resp.Key)
		if err != nil {
			return nil, fmt.Errorf("can't build merkle key path: %w", err)
		}

		// 2) verify value
		err = c.prt.VerifyValue(resp.ProofOps, l.AppHash, kp.String(), resp.Value)
		if err != nil {
			return nil, fmt.Errorf("verify value proof: %w", err)
		}
	} else { // OR validate the absence proof against the trusted header.
		err = c.prt.VerifyAbsence(resp.ProofOps, l.AppHash, string(resp.Key))
		if err != nil {
			return nil, fmt.Errorf("verify absence proof: %w", err)
		}
	}

	return &coretypes.ResultABCIQuery{Response: resp}, nil
}

func (c *Client) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	return c.next.BroadcastTxCommit(ctx, tx)
}

func (c *Client) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTxAsync(ctx, tx)
}

func (c *Client) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTxSync(ctx, tx)
}

func (c *Client) UnconfirmedTxs(ctx context.Context, limit *int) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.next.UnconfirmedTxs(ctx, limit)
}

func (c *Client) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.next.NumUnconfirmedTxs(ctx)
}

func (c *Client) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	return c.next.CheckTx(ctx, tx)
}

func (c *Client) RemoveTx(ctx context.Context, txKey types.TxKey) error {
	return c.next.RemoveTx(ctx, txKey)
}

func (c *Client) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	return c.next.NetInfo(ctx)
}

func (c *Client) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	return c.next.DumpConsensusState(ctx)
}

func (c *Client) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	return c.next.ConsensusState(ctx)
}

func (c *Client) ConsensusParams(ctx context.Context, height *int64) (*coretypes.ResultConsensusParams, error) {
	res, err := c.next.ConsensusParams(ctx, height)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if err := res.ConsensusParams.ValidateConsensusParams(); err != nil {
		return nil, err
	}
	if res.BlockHeight <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.BlockHeight)
	if err != nil {
		return nil, err
	}

	// Verify hash.
	if cH, tH := res.ConsensusParams.HashConsensusParams(), l.ConsensusHash; !bytes.Equal(cH, tH) {
		return nil, fmt.Errorf("params hash %X does not match trusted hash %X",
			cH, tH)
	}

	return res, nil
}

func (c *Client) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	return c.next.Health(ctx)
}

// BlockchainInfo calls rpcclient#BlockchainInfo and then verifies every header
// returned.
func (c *Client) BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) { //nolint:lll
	res, err := c.next.BlockchainInfo(ctx, minHeight, maxHeight)
	if err != nil {
		return nil, err
	}

	// Validate res.
	for i, meta := range res.BlockMetas {
		if meta == nil {
			return nil, fmt.Errorf("nil block meta %d", i)
		}
		if err := meta.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("invalid block meta %d: %w", i, err)
		}
	}

	// Update the light client if we're behind.
	if len(res.BlockMetas) > 0 {
		lastHeight := res.BlockMetas[len(res.BlockMetas)-1].Header.Height
		if _, err := c.updateLightClientIfNeededTo(ctx, &lastHeight); err != nil {
			return nil, err
		}
	}

	// Verify each of the BlockMetas.
	for _, meta := range res.BlockMetas {
		h, err := c.lc.TrustedLightBlock(meta.Header.Height)
		if err != nil {
			return nil, fmt.Errorf("trusted header %d: %w", meta.Header.Height, err)
		}
		if bmH, tH := meta.Header.Hash(), h.Hash(); !bytes.Equal(bmH, tH) {
			return nil, fmt.Errorf("block meta header %X does not match with trusted header %X",
				bmH, tH)
		}
	}

	return res, nil
}

func (c *Client) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	return c.next.Genesis(ctx)
}

func (c *Client) GenesisChunked(ctx context.Context, id uint) (*coretypes.ResultGenesisChunk, error) {
	return c.next.GenesisChunked(ctx, id)
}

// Block calls rpcclient#Block and then verifies the result.
func (c *Client) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	res, err := c.next.Block(ctx, height)
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
		return nil, fmt.Errorf("blockID %X does not match with block %X",
			bmH, bH)
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.Block.Height)
	if err != nil {
		return nil, err
	}

	// Verify block.
	if bH, tH := res.Block.Hash(), l.Hash(); !bytes.Equal(bH, tH) {
		return nil, fmt.Errorf("block header %X does not match with trusted header %X",
			bH, tH)
	}

	return res, nil
}

// BlockByHash calls rpcclient#BlockByHash and then verifies the result.
func (c *Client) BlockByHash(ctx context.Context, hash tmbytes.HexBytes) (*coretypes.ResultBlock, error) {
	res, err := c.next.BlockByHash(ctx, hash)
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
		return nil, fmt.Errorf("blockID %X does not match with block %X",
			bmH, bH)
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.Block.Height)
	if err != nil {
		return nil, err
	}

	// Verify block.
	if bH, tH := res.Block.Hash(), l.Hash(); !bytes.Equal(bH, tH) {
		return nil, fmt.Errorf("block header %X does not match with trusted header %X",
			bH, tH)
	}

	return res, nil
}

// BlockResults returns the block results for the given height. If no height is
// provided, the results of the block preceding the latest are returned.
func (c *Client) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	var h int64
	if height == nil {
		res, err := c.next.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("can't get latest height: %w", err)
		}
		// Can't return the latest block results here because we won't be able to
		// prove them. Return the results for the previous block instead.
		h = res.SyncInfo.LatestBlockHeight - 1
	} else {
		h = *height
	}

	res, err := c.next.BlockResults(ctx, &h)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if res.Height <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	// Update the light client if we're behind.
	nextHeight := h + 1
	trustedBlock, err := c.updateLightClientIfNeededTo(ctx, &nextHeight)
	if err != nil {
		return nil, err
	}

	// proto-encode BeginBlock events
	bbeBytes, err := proto.Marshal(&abci.ResponseBeginBlock{
		Events: res.BeginBlockEvents,
	})
	if err != nil {
		return nil, err
	}

	// Build a Merkle tree of proto-encoded DeliverTx results and get a hash.
	results := types.NewResults(res.TxsResults)

	// proto-encode EndBlock events.
	ebeBytes, err := proto.Marshal(&abci.ResponseEndBlock{
		Events: res.EndBlockEvents,
	})
	if err != nil {
		return nil, err
	}

	// Build a Merkle tree out of the above 3 binary slices.
	rH := merkle.HashFromByteSlices([][]byte{bbeBytes, results.Hash(), ebeBytes})

	// Verify block results.
	if !bytes.Equal(rH, trustedBlock.LastResultsHash) {
		return nil, fmt.Errorf("last results %X does not match with trusted last results %X",
			rH, trustedBlock.LastResultsHash)
	}

	return res, nil
}

func (c *Client) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	// Update the light client if we're behind and retrieve the light block at the requested height
	// or at the latest height if no height is provided.
	l, err := c.updateLightClientIfNeededTo(ctx, height)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultCommit{
		SignedHeader:    *l.SignedHeader,
		CanonicalCommit: true,
	}, nil
}

// Tx calls rpcclient#Tx method and then verifies the proof if such was
// requested.
func (c *Client) Tx(ctx context.Context, hash tmbytes.HexBytes, prove bool) (*coretypes.ResultTx, error) {
	res, err := c.next.Tx(ctx, hash, prove)
	if err != nil || !prove {
		return res, err
	}

	// Validate res.
	if res.Height <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.Height)
	if err != nil {
		return nil, err
	}

	// Validate the proof.
	return res, res.Proof.Validate(l.DataHash)
}

func (c *Client) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {
	return c.next.TxSearch(ctx, query, prove, page, perPage, orderBy)
}

func (c *Client) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {
	return c.next.BlockSearch(ctx, query, page, perPage, orderBy)
}

// Validators fetches and verifies validators.
func (c *Client) Validators(
	ctx context.Context,
	height *int64,
	pagePtr, perPagePtr *int,
) (*coretypes.ResultValidators, error) {

	// Update the light client if we're behind and retrieve the light block at the
	// requested height or at the latest height if no height is provided.
	l, err := c.updateLightClientIfNeededTo(ctx, height)
	if err != nil {
		return nil, err
	}

	totalCount := len(l.ValidatorSet.Validators)
	perPage := validatePerPage(perPagePtr)
	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)
	v := l.ValidatorSet.Validators[skipCount : skipCount+tmmath.MinInt(perPage, totalCount-skipCount)]

	return &coretypes.ResultValidators{
		BlockHeight: l.Height,
		Validators:  v,
		Count:       len(v),
		Total:       totalCount}, nil
}

func (c *Client) BroadcastEvidence(ctx context.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
	return c.next.BroadcastEvidence(ctx, ev)
}

func (c *Client) Subscribe(ctx context.Context, subscriber, query string,
	outCapacity ...int) (out <-chan coretypes.ResultEvent, err error) {
	return c.next.Subscribe(ctx, subscriber, query, outCapacity...)
}

func (c *Client) Unsubscribe(ctx context.Context, subscriber, query string) error {
	return c.next.Unsubscribe(ctx, subscriber, query)
}

func (c *Client) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.next.UnsubscribeAll(ctx, subscriber)
}

func (c *Client) updateLightClientIfNeededTo(ctx context.Context, height *int64) (*types.LightBlock, error) {
	var (
		l   *types.LightBlock
		err error
	)
	if height == nil {
		l, err = c.lc.Update(ctx, time.Now())
	} else {
		l, err = c.lc.VerifyLightBlockAtHeight(ctx, *height, time.Now())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update light client: %w", err)
	}
	return l, nil
}

func (c *Client) RegisterOpDecoder(typ string, dec merkle.OpDecoder) {
	c.prt.RegisterOpDecoder(typ, dec)
}

// SubscribeWS subscribes for events using the given query and remote address as
// a subscriber, but does not verify responses (UNSAFE)!
// TODO: verify data
func (c *Client) SubscribeWS(ctx *rpctypes.Context, query string) (*coretypes.ResultSubscribe, error) {
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
						rpctypes.JSONRPCStringID(fmt.Sprintf("%v#event", ctx.JSONReq.ID)),
						resultEvent,
					))
			case <-c.quitCh:
				return
			}
		}
	}()

	return &coretypes.ResultSubscribe{}, nil
}

// UnsubscribeWS calls original client's Unsubscribe using remote address as a
// subscriber.
func (c *Client) UnsubscribeWS(ctx *rpctypes.Context, query string) (*coretypes.ResultUnsubscribe, error) {
	err := c.next.Unsubscribe(context.Background(), ctx.RemoteAddr(), query)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAllWS calls original client's UnsubscribeAll using remote address
// as a subscriber.
func (c *Client) UnsubscribeAllWS(ctx *rpctypes.Context) (*coretypes.ResultUnsubscribe, error) {
	err := c.next.UnsubscribeAll(context.Background(), ctx.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}

// XXX: Copied from rpc/core/env.go
const (
	// see README
	defaultPerPage = 30
	maxPerPage     = 100
)

func validatePage(pagePtr *int, perPage, totalCount int) (int, error) {
	if perPage < 1 {
		panic(fmt.Errorf("%w (%d)", coretypes.ErrZeroOrNegativePerPage, perPage))
	}

	if pagePtr == nil { // no page parameter
		return 1, nil
	}

	pages := ((totalCount - 1) / perPage) + 1
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	page := *pagePtr
	if page <= 0 || page > pages {
		return 1, fmt.Errorf("%w expected range: [1, %d], given %d", coretypes.ErrPageOutOfRange, pages, page)
	}

	return page, nil
}

func validatePerPage(perPagePtr *int) int {
	if perPagePtr == nil { // no per_page parameter
		return defaultPerPage
	}

	perPage := *perPagePtr
	if perPage < 1 {
		return defaultPerPage
	} else if perPage > maxPerPage {
		return maxPerPage
	}
	return perPage
}

func validateSkipCount(page, perPage int) int {
	skipCount := (page - 1) * perPage
	if skipCount < 0 {
		return 0
	}

	return skipCount
}
