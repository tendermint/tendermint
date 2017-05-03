package mock

import (
	abci "github.com/tendermint/abci/types"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// ABCIApp will send all abci related request to the named app,
// so you can test app behavior from a client without needing
// an entire tendermint node
type ABCIApp struct {
	App abci.Application
}

func (a ABCIApp) _assertABCIClient() client.ABCIClient {
	return a
}

func (a ABCIApp) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return &ctypes.ResultABCIInfo{a.App.Info()}, nil
}

func (a ABCIApp) ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error) {
	q := a.App.Query(abci.RequestQuery{data, path, 0, prove})
	return &ctypes.ResultABCIQuery{q.Result()}, nil
}

func (a ABCIApp) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	res := ctypes.ResultBroadcastTxCommit{}
	res.CheckTx = a.App.CheckTx(tx)
	if !res.CheckTx.IsOK() {
		return &res, nil
	}
	res.DeliverTx = a.App.DeliverTx(tx)
	return &res, nil
}

func (a ABCIApp) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	c := a.App.CheckTx(tx)
	// and this gets writen in a background thread...
	if c.IsOK() {
		go func() { a.App.DeliverTx(tx) }()
	}
	return &ctypes.ResultBroadcastTx{c.Code, c.Data, c.Log, tx.Hash()}, nil
}

func (a ABCIApp) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	c := a.App.CheckTx(tx)
	// and this gets writen in a background thread...
	if c.IsOK() {
		go func() { a.App.DeliverTx(tx) }()
	}
	return &ctypes.ResultBroadcastTx{c.Code, c.Data, c.Log, tx.Hash()}, nil
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

func (m ABCIMock) _assertABCIClient() client.ABCIClient {
	return m
}

func (m ABCIMock) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	res, err := m.Info.GetResponse(nil)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIInfo{res.(abci.ResponseInfo)}, nil
}

func (m ABCIMock) ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error) {
	res, err := m.Query.GetResponse(QueryArgs{path, data, prove})
	if err != nil {
		return nil, err
	}
	resQuery := res.(abci.ResponseQuery)
	return &ctypes.ResultABCIQuery{resQuery.Result()}, nil
}

func (m ABCIMock) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	res, err := m.BroadcastCommit.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*ctypes.ResultBroadcastTxCommit), nil
}

func (m ABCIMock) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	res, err := m.Broadcast.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*ctypes.ResultBroadcastTx), nil
}

func (m ABCIMock) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	res, err := m.Broadcast.GetResponse(tx)
	if err != nil {
		return nil, err
	}
	return res.(*ctypes.ResultBroadcastTx), nil
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

func (r *ABCIRecorder) _assertABCIClient() client.ABCIClient {
	return r
}

type QueryArgs struct {
	Path  string
	Data  data.Bytes
	Prove bool
}

func (r *ABCIRecorder) addCall(call Call) {
	r.Calls = append(r.Calls, call)
}

func (r *ABCIRecorder) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	res, err := r.Client.ABCIInfo()
	r.addCall(Call{
		Name:     "abci_info",
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error) {
	res, err := r.Client.ABCIQuery(path, data, prove)
	r.addCall(Call{
		Name:     "abci_query",
		Args:     QueryArgs{path, data, prove},
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	res, err := r.Client.BroadcastTxCommit(tx)
	r.addCall(Call{
		Name:     "broadcast_tx_commit",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTxAsync(tx)
	r.addCall(Call{
		Name:     "broadcast_tx_async",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}

func (r *ABCIRecorder) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTxSync(tx)
	r.addCall(Call{
		Name:     "broadcast_tx_sync",
		Args:     tx,
		Response: res,
		Error:    err,
	})
	return res, err
}
