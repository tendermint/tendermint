package proxy

import (
	"github.com/jinzhu/copier"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
	abcixcli "github.com/tendermint/tendermint/abcix/client"
	xtypes "github.com/tendermint/tendermint/abcix/types"
)

//go:generate mockery -case underscore -name AppConnConsensus|AppConnMempool|AppConnQuery|AppConnSnapshot

//----------------------------------------------------------------------------------------
// Enforce which abci msgs can be sent on a connection at the type level

type LegacyAppConnConsensus interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	InitChainSync(types.RequestInitChain) (*types.ResponseInitChain, error)

	BeginBlockSync(types.RequestBeginBlock) (*types.ResponseBeginBlock, error)
	DeliverTxAsync(types.RequestDeliverTx) *abcicli.ReqRes
	EndBlockSync(types.RequestEndBlock) (*types.ResponseEndBlock, error)
	CommitSync() (*types.ResponseCommit, error)
}

type LegacyAppConnMempool interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	CheckTxAsync(types.RequestCheckTx) *abcicli.ReqRes
	CheckTxSync(types.RequestCheckTx) (*types.ResponseCheckTx, error)

	FlushAsync() *abcicli.ReqRes
	FlushSync() error
}

type LegacyAppConnQuery interface {
	Error() error

	EchoSync(string) (*types.ResponseEcho, error)
	InfoSync(types.RequestInfo) (*types.ResponseInfo, error)
	QuerySync(types.RequestQuery) (*types.ResponseQuery, error)

	//	SetOptionSync(key string, value string) (res types.Result)
}

type LegacyAppConnSnapshot interface {
	Error() error

	ListSnapshotsSync(types.RequestListSnapshots) (*types.ResponseListSnapshots, error)
	OfferSnapshotSync(types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error)
	LoadSnapshotChunkSync(types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunkSync(types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of abcicli.Client)

type legacyAppConnConsensus struct {
	appConn abcicli.Client
}

func NewLegacyAppConnConsensus(appConn abcicli.Client) LegacyAppConnConsensus {
	return &legacyAppConnConsensus{
		appConn: appConn,
	}
}

func (app *legacyAppConnConsensus) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *legacyAppConnConsensus) Error() error {
	return app.appConn.Error()
}

func (app *legacyAppConnConsensus) InitChainSync(req types.RequestInitChain) (*types.ResponseInitChain, error) {
	return app.appConn.InitChainSync(req)
}

func (app *legacyAppConnConsensus) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	return app.appConn.BeginBlockSync(req)
}

func (app *legacyAppConnConsensus) DeliverTxAsync(req types.RequestDeliverTx) *abcicli.ReqRes {
	return app.appConn.DeliverTxAsync(req)
}

func (app *legacyAppConnConsensus) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	return app.appConn.EndBlockSync(req)
}

func (app *legacyAppConnConsensus) CommitSync() (*types.ResponseCommit, error) {
	return app.appConn.CommitSync()
}

//------------------------------------------------
// Implements AppConnMempool (subset of abcicli.Client)

type legacyAppConnMempool struct {
	appConn abcicli.Client
}

func NewLegacyAppConnMempool(appConn abcicli.Client) LegacyAppConnMempool {
	return &legacyAppConnMempool{
		appConn: appConn,
	}
}

func (app *legacyAppConnMempool) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *legacyAppConnMempool) Error() error {
	return app.appConn.Error()
}

func (app *legacyAppConnMempool) FlushAsync() *abcicli.ReqRes {
	return app.appConn.FlushAsync()
}

func (app *legacyAppConnMempool) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *legacyAppConnMempool) CheckTxAsync(req types.RequestCheckTx) *abcicli.ReqRes {
	return app.appConn.CheckTxAsync(req)
}

func (app *legacyAppConnMempool) CheckTxSync(req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	return app.appConn.CheckTxSync(req)
}

// Helper struct to adapt legacy mempool app conn
type adaptedLegacyAppConnMempool struct {
	legacyApp LegacyAppConnMempool
}

func (app *adaptedLegacyAppConnMempool) SetResponseCallback(callback abcixcli.Callback) {
	app.legacyApp.SetResponseCallback(func(req *types.Request, resp *types.Response) {
		// Right now, global callback is only used for tx recheck, thus we only handle if it's checkTx
		if _, ok := resp.Value.(*types.Response_CheckTx); !ok {
			return
		}

		var checkTxReq xtypes.RequestCheckTx
		if err := copier.Copy(&checkTxReq, req.GetCheckTx()); err != nil {
			// TODO: panic for debugging purposes. better error handling soon!
			panic(err)
		}
		newReq := xtypes.ToRequestCheckTx(checkTxReq)

		var checkTxResp xtypes.ResponseCheckTx
		if err := copier.Copy(&checkTxResp, resp.GetCheckTx()); err != nil {
			// TODO: panic for debugging purposes. better error handling soon!
			panic(err)
		}
		newResp := xtypes.ToResponseCheckTx(checkTxResp)

		callback(newReq, newResp)
	})
}

func (app *adaptedLegacyAppConnMempool) Error() error {
	return app.legacyApp.Error()
}

func (app *adaptedLegacyAppConnMempool) CheckTxAsync(req xtypes.RequestCheckTx) *abcixcli.ReqRes {
	legacyReq := types.RequestCheckTx{}
	if err := copier.Copy(&legacyReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	legacyReqRes := app.legacyApp.CheckTxAsync(legacyReq)

	reqRes := abcixcli.NewReqRes(xtypes.ToRequestCheckTx(req))
	reqRes.WaitGroup = legacyReqRes.WaitGroup
	// Note here we only adapt for local client, thus response should be ready already
	if legacyReqRes.Response == nil || legacyReqRes.Response.GetCheckTx() == nil {
		panic("unexpected legacy app checkTx async result, shouldn't have nil checkTx response")
	}
	var resp xtypes.ResponseCheckTx
	if err := copier.Copy(&resp, legacyReqRes.Response.GetCheckTx()); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	reqRes.Response = xtypes.ToResponseCheckTx(resp)
	reqRes.SetDone()
	return reqRes
}

func (app *adaptedLegacyAppConnMempool) CheckTxSync(req xtypes.RequestCheckTx) (*xtypes.ResponseCheckTx, error) {
	legacyReq := types.RequestCheckTx{}
	if err := copier.Copy(&legacyReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp, err := app.legacyApp.CheckTxSync(legacyReq)
	if err != nil {
		return nil, err
	}
	resp := &xtypes.ResponseCheckTx{}
	if err := copier.Copy(resp, abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return resp, nil
}

func (app *adaptedLegacyAppConnMempool) FlushAsync() *abcixcli.ReqRes {
	// Note here we only adapt for local client, which does nothing in this method
	reqRes := abcixcli.NewReqRes(xtypes.ToRequestFlush())
	reqRes.SetDone()
	return reqRes
}

func (app *adaptedLegacyAppConnMempool) FlushSync() error {
	return app.legacyApp.FlushSync()
}

// AdaptLegacy will convert a legacy mempool app conn to the one supporting ABCIx
func AdaptLegacy(legacyApp LegacyAppConnMempool) AppConnMempool {
	return &adaptedLegacyAppConnMempool{legacyApp: legacyApp}
}

//------------------------------------------------
// Implements AppConnQuery (subset of abcicli.Client)

type legacyAppConnQuery struct {
	appConn abcicli.Client
}

func NewLegacyAppConnQuery(appConn abcicli.Client) LegacyAppConnQuery {
	return &legacyAppConnQuery{
		appConn: appConn,
	}
}

func (app *legacyAppConnQuery) Error() error {
	return app.appConn.Error()
}

func (app *legacyAppConnQuery) EchoSync(msg string) (*types.ResponseEcho, error) {
	return app.appConn.EchoSync(msg)
}

func (app *legacyAppConnQuery) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	return app.appConn.InfoSync(req)
}

func (app *legacyAppConnQuery) QuerySync(reqQuery types.RequestQuery) (*types.ResponseQuery, error) {
	return app.appConn.QuerySync(reqQuery)
}

//------------------------------------------------
// Implements AppConnSnapshot (subset of abcicli.Client)

type legacyAppConnSnapshot struct {
	appConn abcicli.Client
}

func NewLegacyAppConnSnapshot(appConn abcicli.Client) LegacyAppConnSnapshot {
	return &legacyAppConnSnapshot{
		appConn: appConn,
	}
}

func (app *legacyAppConnSnapshot) Error() error {
	return app.appConn.Error()
}

func (app *legacyAppConnSnapshot) ListSnapshotsSync(
	req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	return app.appConn.ListSnapshotsSync(req)
}

func (app *legacyAppConnSnapshot) OfferSnapshotSync(
	req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	return app.appConn.OfferSnapshotSync(req)
}

func (app *legacyAppConnSnapshot) LoadSnapshotChunkSync(
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	return app.appConn.LoadSnapshotChunkSync(req)
}

func (app *legacyAppConnSnapshot) ApplySnapshotChunkSync(
	req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	return app.appConn.ApplySnapshotChunkSync(req)
}
