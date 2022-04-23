package mock

/*
package mock returns a Client implementation that
accepts various (mock) implementations of the various methods.

This implementation is useful for using in tests, when you don't
need a real server, but want a high-level of control about
the server response you want to mock (eg. error handling),
or if you just want to record the calls to verify in your tests.

For real clients, you probably want the "http" package.  If you
want to directly call a tendermint node in process, you can use the
"local" package.
*/

import (
	"context"
	"reflect"

	"github.com/tendermint/tendermint/internal/rpc/core"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

// Client wraps arbitrary implementations of the various interfaces.
type Client struct {
	client.Client
	env *core.Environment
}

func New() Client {
	return Client{
		env: &core.Environment{},
	}
}

var _ client.Client = Client{}

// Call is used by recorders to save a call and response.
// It can also be used to configure mock responses.
//
type Call struct {
	Name     string
	Args     interface{}
	Response interface{}
	Error    error
}

// GetResponse will generate the apporiate response for us, when
// using the Call struct to configure a Mock handler.
//
// When configuring a response, if only one of Response or Error is
// set then that will always be returned. If both are set, then
// we return Response if the Args match the set args, Error otherwise.
func (c Call) GetResponse(args interface{}) (interface{}, error) {
	// handle the case with no response
	if c.Response == nil {
		if c.Error == nil {
			panic("Misconfigured call, you must set either Response or Error")
		}
		return nil, c.Error
	}
	// response without error
	if c.Error == nil {
		return c.Response, nil
	}
	// have both, we must check args....
	if reflect.DeepEqual(args, c.Args) {
		return c.Response, nil
	}
	return nil, c.Error
}

func (c Client) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	return c.env.Status(ctx)
}

func (c Client) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	return c.env.ABCIInfo(ctx)
}

func (c Client) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, path, data, client.DefaultABCIQueryOptions)
}

func (c Client) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts client.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	return c.env.ABCIQuery(ctx, &coretypes.RequestABCIQuery{
		Path: path, Data: data, Height: coretypes.Int64(opts.Height), Prove: opts.Prove,
	})
}

func (c Client) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	return c.env.BroadcastTxCommit(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c Client) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTxAsync(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c Client) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTxSync(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c Client) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	return c.env.CheckTx(ctx, &coretypes.RequestCheckTx{Tx: tx})
}

func (c Client) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	return c.env.NetInfo(ctx)
}

func (c Client) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	return c.env.GetConsensusState(ctx)
}

func (c Client) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	return c.env.DumpConsensusState(ctx)
}

func (c Client) ConsensusParams(ctx context.Context, height *int64) (*coretypes.ResultConsensusParams, error) {
	return c.env.ConsensusParams(ctx, &coretypes.RequestConsensusParams{Height: (*coretypes.Int64)(height)})
}

func (c Client) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	return c.env.Health(ctx)
}

func (c Client) BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) {
	return c.env.BlockchainInfo(ctx, &coretypes.RequestBlockchainInfo{
		MinHeight: coretypes.Int64(minHeight),
		MaxHeight: coretypes.Int64(maxHeight),
	})
}

func (c Client) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	return c.env.Genesis(ctx)
}

func (c Client) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	return c.env.Block(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(height)})
}

func (c Client) BlockByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error) {
	return c.env.BlockByHash(ctx, &coretypes.RequestBlockByHash{Hash: hash})
}

func (c Client) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	return c.env.Commit(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(height)})
}

func (c Client) Validators(ctx context.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error) {
	return c.env.Validators(ctx, &coretypes.RequestValidators{
		Height:  (*coretypes.Int64)(height),
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	})
}

func (c Client) BroadcastEvidence(ctx context.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
	return c.env.BroadcastEvidence(ctx, &coretypes.RequestBroadcastEvidence{Evidence: ev})
}
