package types

import (
	context "golang.org/x/net/context"
)

// Applications
type Application interface {
	// Info/Query Connection
	Info() ResponseInfo                              // Return application info
	SetOption(key string, value string) (log string) // Set application option
	Query(reqQuery RequestQuery) ResponseQuery       // Query for state

	// Mempool Connection
	CheckTx(tx []byte) Result // Validate a tx for the mempool

	// Consensus Connection
	InitChain(validators []*Validator)       // Initialize blockchain with validators from TendermintCore
	BeginBlock(hash []byte, header *Header)  // Signals the beginning of a block
	DeliverTx(tx []byte) Result              // Deliver a tx for full processing
	EndBlock(height uint64) ResponseEndBlock // Signals the end of a block, returns changes to the validator set
	Commit() Result                          // Commit the state and return the application Merkle root hash
}

//------------------------------------

// GRPC wrapper for application
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
	resQuery := app.app.Query(*req)
	return &resQuery, nil
}

func (app *GRPCApplication) Commit(ctx context.Context, req *RequestCommit) (*ResponseCommit, error) {
	r := app.app.Commit()
	return &ResponseCommit{r.Code, r.Data, r.Log}, nil
}

func (app *GRPCApplication) InitChain(ctx context.Context, req *RequestInitChain) (*ResponseInitChain, error) {
	app.app.InitChain(req.Validators)
	return &ResponseInitChain{}, nil // NOTE: empty return
}

func (app *GRPCApplication) BeginBlock(ctx context.Context, req *RequestBeginBlock) (*ResponseBeginBlock, error) {
	app.app.BeginBlock(req.Hash, req.Header)
	return &ResponseBeginBlock{}, nil // NOTE: empty return
}

func (app *GRPCApplication) EndBlock(ctx context.Context, req *RequestEndBlock) (*ResponseEndBlock, error) {
	resEndBlock := app.app.EndBlock(req.Height)
	return &resEndBlock, nil
}
