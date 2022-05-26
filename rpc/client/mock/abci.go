package mock

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

// ABCIApp will send all abci related request to the named app,
// so you can test app behavior from a client without needing
// an entire tendermint node
type ABCIApp struct {
	App abci.Application
}

var (
	_ client.ABCIClient = ABCIApp{}
	_ client.ABCIClient = ABCIMock{}
	_ client.ABCIClient = (*ABCIRecorder)(nil)
)

func (a ABCIApp) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	res, err := a.App.Info(ctx, &proxy.RequestInfo)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIInfo{Response: *res}, nil
}

func (a ABCIApp) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return a.ABCIQueryWithOptions(ctx, path, data, client.DefaultABCIQueryOptions)
}

func (a ABCIApp) ABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts client.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	q, err := a.App.Query(ctx, &abci.RequestQuery{
		Data:   data,
		Path:   path,
		Height: opts.Height,
		Prove:  opts.Prove,
	})
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIQuery{Response: *q}, nil
}

// NOTE: Caller should call a.App.Commit() separately,
// this function does not actually wait for a commit.
// TODO: Make it wait for a commit and set res.Height appropriately.
func (a ABCIApp) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	resp, err := a.App.CheckTx(ctx, &abci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}

	res := &coretypes.ResultBroadcastTxCommit{CheckTx: *resp}
	if res.CheckTx.IsErr() {
		return res, nil
	}

	fb, err := a.App.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx}})
	if err != nil {
		return nil, err
	}

	res.TxResult = *fb.TxResults[0]
	res.Height = -1 // TODO
	return res, nil
}

func (a ABCIApp) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	c, err := a.App.CheckTx(ctx, &abci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}

	// and this gets written in a background thread...
	if !c.IsErr() {
		go func() { _, _ = a.App.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx}}) }()
	}
	return &coretypes.ResultBroadcastTx{
		Code:      c.Code,
		Data:      c.Data,
		Codespace: c.Codespace,
		Hash:      tx.Hash(),
	}, nil
}

func (a ABCIApp) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return a.BroadcastTx(ctx, tx)
}

func (a ABCIApp) BroadcastTx(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	c, err := a.App.CheckTx(ctx, &abci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}

	// and this gets written in a background thread...
	if !c.IsErr() {
		go func() { _, _ = a.App.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx}}) }()
	}
	return &coretypes.ResultBroadcastTx{
		Code:      c.Code,
		Data:      c.Data,
		Codespace: c.Codespace,
		Hash:      tx.Hash(),
	}, nil
}

// ABCIMock will send all abci related request to the named app,
// so you can test app behavior from a client without needing
// an entire tendermint node
type ABCIMock struct {
	Info            Call
	Query           Call
	BroadcastCommit Call
	Broadcast       Call
}

func (m ABCIMock) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	res, err := m.Info.GetResponse(nil)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultABCIInfo{Response: res.(abci.ResponseInfo)}, nil
}

func (m ABCIMock) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return m.ABCIQueryWithOptions(ctx, path, data, client.DefaultABCIQueryOptions)
}

func (m ABCIMock) ABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts client.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	res, err := m.Query.GetResponse(QueryArgs{path, data, opts.Height, opts.Prove})
	if err != nil {
		return nil, err
	}
	resQuery := res.(abci.ResponseQuery)
	return &coretypes.ResultABCIQuery{Response: resQuery}, nil
}

func (m ABCIMock) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	res, err := m.BroadcastCommit.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*coretypes.ResultBroadcastTxCommit), nil
}

func (m ABCIMock) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := m.Broadcast.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*coretypes.ResultBroadcastTx), nil
}

func (m ABCIMock) BroadcastTx(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := m.Broadcast.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*coretypes.ResultBroadcastTx), nil
}

func (m ABCIMock) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := m.Broadcast.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*coretypes.ResultBroadcastTx), nil
}

// ABCIRecorder can wrap another type (ABCIApp, ABCIMock, or Client)
// and record all ABCI related calls.
type ABCIRecorder struct {
	Client client.ABCIClient
	Calls  []Call
}

func NewABCIRecorder(client client.ABCIClient) *ABCIRecorder {
	return &ABCIRecorder{
		Client: client,
		Calls:  []Call{},
	}
}

type QueryArgs struct {
	Path   string
	Data   bytes.HexBytes
	Height int64
	Prove  bool
}

func (r *ABCIRecorder) addCall(call Call) {
	r.Calls = append(r.Calls, call)
}

func (r *ABCIRecorder) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	res, err := r.Client.ABCIInfo(ctx)
	r.addCall(Call{
		Name:     "abci_info",
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return r.ABCIQueryWithOptions(ctx, path, data, client.DefaultABCIQueryOptions)
}

func (r *ABCIRecorder) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts client.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	res, err := r.Client.ABCIQueryWithOptions(ctx, path, data, opts)
	r.addCall(Call{
		Name:     "abci_query",
		Args:     QueryArgs{path, data, opts.Height, opts.Prove},
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	res, err := r.Client.BroadcastTxCommit(ctx, tx)
	r.addCall(Call{
		Name:     "broadcast_tx_commit",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTxAsync(ctx, tx)
	r.addCall(Call{
		Name:     "broadcast_tx_async",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTxSync(ctx, tx)
	r.addCall(Call{
		Name:     "broadcast_tx_sync",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTx(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTx(ctx, tx)
	r.addCall(Call{
		Name:     "broadcast_tx",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}
