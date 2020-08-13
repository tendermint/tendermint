package adapter

import (
	"github.com/jinzhu/copier"

	abci "github.com/tendermint/tendermint/abci/types"
	abcix "github.com/tendermint/tendermint/abcix/types"
)

type adaptedApp struct {
	abciApp abci.Application
}

type AdaptedApp interface {
	OriginalApp() abci.Application
}

func (app *adaptedApp) OriginalApp() abci.Application {
	return app.abciApp
}

func (app *adaptedApp) Info(req abcix.RequestInfo) (resp abcix.ResponseInfo) {
	abciReq := abci.RequestInfo{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.Info(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) SetOption(req abcix.RequestSetOption) (resp abcix.ResponseSetOption) {
	abciReq := abci.RequestSetOption{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.SetOption(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) Query(req abcix.RequestQuery) (resp abcix.ResponseQuery) {
	abciReq := abci.RequestQuery{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.Query(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) CheckTx(req abcix.RequestCheckTx) (resp abcix.ResponseCheckTx) {
	abciReq := abci.RequestCheckTx{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.CheckTx(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) CreateBlock(req abcix.RequestCreateBlock, iter *abcix.MempoolIter) abcix.ResponseCreateBlock {
	// TODO: defer to consensus engine for now
	panic("implement me")
}

func (app *adaptedApp) InitChain(req abcix.RequestInitChain) (resp abcix.ResponseInitChain) {
	abciReq := abci.RequestInitChain{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.InitChain(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) DeliverBlock(req abcix.RequestDeliverBlock) (resp abcix.ResponseDeliverBlock) {
	var reqBegin abci.RequestBeginBlock
	if err := copier.Copy(&reqBegin, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	respBegin := app.abciApp.BeginBlock(reqBegin)
	events := respBegin.Events
	if err := copier.Copy(&resp, &respBegin); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}

	for _, tx := range req.Txs {
		legacyRespDeliverTx := app.abciApp.DeliverTx(abci.RequestDeliverTx{Tx: tx})
		var respDeliverTx abcix.ResponseDeliverTx
		if err := copier.Copy(&respDeliverTx, &legacyRespDeliverTx); err != nil {
			// TODO: panic for debugging purposes. better error handling soon!
			panic(err)
		}
		resp.DeliverTxs = append(resp.DeliverTxs, &respDeliverTx)
	}

	var reqEnd abci.RequestEndBlock
	if err := copier.Copy(&reqEnd, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	respEnd := app.abciApp.EndBlock(reqEnd)
	events = append(events, respEnd.Events...)
	if err := copier.Copy(&resp, &respEnd); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}

	// Reform the events
	resp.Events = make([]abcix.Event, len(events))
	for i, oldEvent := range events {
		oldEvent := oldEvent
		var newEvent abcix.Event
		if err := copier.Copy(&newEvent, &oldEvent); err != nil {
			// TODO: panic for debugging purposes. better error handling soon!
			panic(err)
		}
		resp.Events[i] = newEvent
	}

	return resp
}

func (app *adaptedApp) Commit() (resp abcix.ResponseCommit) {
	abciResp := app.abciApp.Commit()
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) ListSnapshots(req abcix.RequestListSnapshots) (resp abcix.ResponseListSnapshots) {
	abciReq := abci.RequestListSnapshots{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.ListSnapshots(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) OfferSnapshot(req abcix.RequestOfferSnapshot) (resp abcix.ResponseOfferSnapshot) {
	abciReq := abci.RequestOfferSnapshot{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.OfferSnapshot(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) LoadSnapshotChunk(req abcix.RequestLoadSnapshotChunk) (resp abcix.ResponseLoadSnapshotChunk) {
	abciReq := abci.RequestLoadSnapshotChunk{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.LoadSnapshotChunk(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func (app *adaptedApp) ApplySnapshotChunk(req abcix.RequestApplySnapshotChunk) (resp abcix.ResponseApplySnapshotChunk) {
	abciReq := abci.RequestApplySnapshotChunk{}
	if err := copier.Copy(&abciReq, &req); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	abciResp := app.abciApp.ApplySnapshotChunk(abciReq)
	if err := copier.Copy(&resp, &abciResp); err != nil {
		// TODO: panic for debugging purposes. better error handling soon!
		panic(err)
	}
	return
}

func AdaptToABCIx(abciApp abci.Application) abcix.Application {
	return &adaptedApp{abciApp: abciApp}
}
