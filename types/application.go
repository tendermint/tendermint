package types

import (
	context "golang.org/x/net/context"
)

// Applications
type Application interface {

	// Return application info
	Info() ResponseInfo

	// Set application option (e.g. mode=mempool, mode=consensus)
	SetOption(key string, value string) (log string)

	// Deliver a tx
	DeliverTx(tx []byte) Result

	// Validate a tx for the mempool
	CheckTx(tx []byte) Result

	// Query for state
	Query(query []byte) Result

	// Return the application Merkle root hash
	Commit() Result
}

// Some applications can choose to implement BlockchainAware
type BlockchainAware interface {

	// Initialize blockchain
	// validators: genesis validators from TendermintCore
	InitChain(validators []*Validator)

	// Signals the beginning of a block
	BeginBlock(hash []byte, header *Header)

	// Signals the end of a block
	// diffs: changed validators from app to TendermintCore
	EndBlock(height uint64) ResponseEndBlock
}

//------------------------------------
type GRPCApplication struct {
	app Application
}

func NewGRPCApplication(app Application) *GRPCApplication {
	return &GRPCApplication{app}
}

func (app *GRPCApplication) Echo(ctx context.Context, req *RequestEcho) (*ResponseEcho, error) {
	return &ResponseEcho{req.Message}, nil
}

func (app *GRPCApplication) Flush(ctx context.Context, req *RequestFlush) (*ResponseFlush, error) {
	return &ResponseFlush{}, nil
}

func (app *GRPCApplication) Info(ctx context.Context, req *RequestInfo) (*ResponseInfo, error) {
	resInfo := app.app.Info()
	return &resInfo, nil
}

func (app *GRPCApplication) SetOption(ctx context.Context, req *RequestSetOption) (*ResponseSetOption, error) {
	return &ResponseSetOption{app.app.SetOption(req.Key, req.Value)}, nil
}

func (app *GRPCApplication) DeliverTx(ctx context.Context, req *RequestDeliverTx) (*ResponseDeliverTx, error) {
	r := app.app.DeliverTx(req.Tx)
	return &ResponseDeliverTx{r.Code, r.Data, r.Log}, nil
}

func (app *GRPCApplication) CheckTx(ctx context.Context, req *RequestCheckTx) (*ResponseCheckTx, error) {
	r := app.app.CheckTx(req.Tx)
	return &ResponseCheckTx{r.Code, r.Data, r.Log}, nil
}

func (app *GRPCApplication) Query(ctx context.Context, req *RequestQuery) (*ResponseQuery, error) {
	r := app.app.Query(req.Query)
	return &ResponseQuery{r.Code, r.Data, r.Log}, nil
}

func (app *GRPCApplication) Commit(ctx context.Context, req *RequestCommit) (*ResponseCommit, error) {
	r := app.app.Commit()
	return &ResponseCommit{r.Code, r.Data, r.Log}, nil
}

func (app *GRPCApplication) InitChain(ctx context.Context, req *RequestInitChain) (*ResponseInitChain, error) {
	if chainAware, ok := app.app.(BlockchainAware); ok {
		chainAware.InitChain(req.Validators)
	}
	return &ResponseInitChain{}, nil
}

func (app *GRPCApplication) BeginBlock(ctx context.Context, req *RequestBeginBlock) (*ResponseBeginBlock, error) {
	if chainAware, ok := app.app.(BlockchainAware); ok {
		chainAware.BeginBlock(req.Hash, req.Header)
	}
	return &ResponseBeginBlock{}, nil
}

func (app *GRPCApplication) EndBlock(ctx context.Context, req *RequestEndBlock) (*ResponseEndBlock, error) {
	if chainAware, ok := app.app.(BlockchainAware); ok {
		resEndBlock := chainAware.EndBlock(req.Height)
		return &resEndBlock, nil
	}
	return &ResponseEndBlock{}, nil
}
