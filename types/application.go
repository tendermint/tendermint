package types // nolint: goimports

import (
	context "golang.org/x/net/context"
)

// Application is an interface that enables any finite, deterministic state machine
// to be driven by a blockchain-based replication engine via the ABCI.
// All methods take a ParamsXxx argument and return a ResultXxx argument,
// except CheckTx/DeliverTx, which take `tx []byte`, and `Commit`, which takes nothing.
type Application interface {
	// Info/Query Connection
	Info(ParamsInfo) ResultInfo                // Return application info
	SetOption(ParamsSetOption) ResultSetOption // Set application option
	Query(ParamsQuery) ResultQuery             // Query for state

	// Mempool Connection
	CheckTx(tx []byte) ResultCheckTx // Validate a tx for the mempool

	// Consensus Connection
	InitChain(ParamsInitChain) ResultInitChain    // Initialize blockchain with validators and other info from TendermintCore
	BeginBlock(ParamsBeginBlock) ResultBeginBlock // Signals the beginning of a block
	DeliverTx(tx []byte) ResultDeliverTx          // Deliver a tx for full processing
	EndBlock(ParamsEndBlock) ResultEndBlock       // Signals the end of a block, returns changes to the validator set
	Commit() ResultCommit                         // Commit the state and return the application Merkle root hash
}

//-------------------------------------------------------
// BaseApplication is a base form of Application

var _ Application = (*BaseApplication)(nil)

type BaseApplication struct {
}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(req ParamsInfo) ResultInfo {
	return ResultInfo{}
}

func (BaseApplication) SetOption(req ParamsSetOption) ResultSetOption {
	return ResultSetOption{}
}

func (BaseApplication) DeliverTx(tx []byte) ResultDeliverTx {
	return ResultDeliverTx{Code: CodeTypeOK}
}

func (BaseApplication) CheckTx(tx []byte) ResultCheckTx {
	return ResultCheckTx{Code: CodeTypeOK}
}

func (BaseApplication) Commit() ResultCommit {
	return ResultCommit{}
}

func (BaseApplication) Query(req ParamsQuery) ResultQuery {
	return ResultQuery{Code: CodeTypeOK}
}

func (BaseApplication) InitChain(req ParamsInitChain) ResultInitChain {
	return ResultInitChain{}
}

func (BaseApplication) BeginBlock(req ParamsBeginBlock) ResultBeginBlock {
	return ResultBeginBlock{}
}

func (BaseApplication) EndBlock(req ParamsEndBlock) ResultEndBlock {
	return ResultEndBlock{}
}

//-------------------------------------------------------

// GRPCApplication is a GRPC wrapper for Application
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
	res := app.app.Info(ToParamsInfo(*req))
	r := FromResultInfo(res)
	return &r, nil
}

func (app *GRPCApplication) SetOption(ctx context.Context, req *RequestSetOption) (*ResponseSetOption, error) {
	res := app.app.SetOption(ToParamsSetOption(*req))
	r := FromResultSetOption(res)
	return &r, nil
}

func (app *GRPCApplication) DeliverTx(ctx context.Context, req *RequestDeliverTx) (*ResponseDeliverTx, error) {
	res := app.app.DeliverTx(req.Tx)
	r := FromResultDeliverTx(res)
	return &r, nil
}

func (app *GRPCApplication) CheckTx(ctx context.Context, req *RequestCheckTx) (*ResponseCheckTx, error) {
	res := app.app.CheckTx(req.Tx)
	r := FromResultCheckTx(res)
	return &r, nil
}

func (app *GRPCApplication) Query(ctx context.Context, req *RequestQuery) (*ResponseQuery, error) {
	res := app.app.Query(ToParamsQuery(*req))
	r := FromResultQuery(res)
	return &r, nil
}

func (app *GRPCApplication) Commit(ctx context.Context, req *RequestCommit) (*ResponseCommit, error) {
	res := app.app.Commit()
	r := FromResultCommit(res)
	return &r, nil
}

func (app *GRPCApplication) InitChain(ctx context.Context, req *RequestInitChain) (*ResponseInitChain, error) {
	res := app.app.InitChain(ToParamsInitChain(*req))
	r := FromResultInitChain(res)
	return &r, nil
}

func (app *GRPCApplication) BeginBlock(ctx context.Context, req *RequestBeginBlock) (*ResponseBeginBlock, error) {
	res := app.app.BeginBlock(ToParamsBeginBlock(*req))
	r := FromResultBeginBlock(res)
	return &r, nil
}

func (app *GRPCApplication) EndBlock(ctx context.Context, req *RequestEndBlock) (*ResponseEndBlock, error) {
	res := app.app.EndBlock(ToParamsEndBlock(*req))
	r := FromResultEndBlock(res)
	return &r, nil
}
