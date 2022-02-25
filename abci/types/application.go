package types

import (
	"context"
)

//go:generate ../../scripts/mockery_generate.sh Application
// Application is an interface that enables any finite, deterministic state machine
// to be driven by a blockchain-based replication engine via the ABCI.
// All methods take a RequestXxx argument and return a ResponseXxx argument,
// except CheckTx/DeliverTx, which take `tx []byte`, and `Commit`, which takes nothing.
type Application interface {
	// Info/Query Connection
	Info(RequestInfo) ResponseInfo    // Return application info
	Query(RequestQuery) ResponseQuery // Query for state

	// Mempool Connection
	CheckTx(RequestCheckTx) ResponseCheckTx // Validate a tx for the mempool

	// Consensus Connection
	InitChain(RequestInitChain) ResponseInitChain // Initialize blockchain w validators/other info from TendermintCore
	PrepareProposal(RequestPrepareProposal) ResponsePrepareProposal
	ProcessProposal(RequestProcessProposal) ResponseProcessProposal
	// Commit the state and return the application Merkle root hash
	Commit() ResponseCommit
	// Create application specific vote extension
	ExtendVote(RequestExtendVote) ResponseExtendVote
	// Verify application's vote extension data
	VerifyVoteExtension(RequestVerifyVoteExtension) ResponseVerifyVoteExtension
	// Deliver the decided block with its txs to the Application
	FinalizeBlock(RequestFinalizeBlock) ResponseFinalizeBlock

	// State Sync Connection
	ListSnapshots(RequestListSnapshots) ResponseListSnapshots                // List available snapshots
	OfferSnapshot(RequestOfferSnapshot) ResponseOfferSnapshot                // Offer a snapshot to the application
	LoadSnapshotChunk(RequestLoadSnapshotChunk) ResponseLoadSnapshotChunk    // Load a snapshot chunk
	ApplySnapshotChunk(RequestApplySnapshotChunk) ResponseApplySnapshotChunk // Apply a shapshot chunk
}

//-------------------------------------------------------
// BaseApplication is a base form of Application

var _ Application = (*BaseApplication)(nil)

type BaseApplication struct{}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(req RequestInfo) ResponseInfo {
	return ResponseInfo{}
}

func (BaseApplication) CheckTx(req RequestCheckTx) ResponseCheckTx {
	return ResponseCheckTx{Code: CodeTypeOK}
}

func (BaseApplication) Commit() ResponseCommit {
	return ResponseCommit{}
}

func (BaseApplication) ExtendVote(req RequestExtendVote) ResponseExtendVote {
	return ResponseExtendVote{}
}

func (BaseApplication) VerifyVoteExtension(req RequestVerifyVoteExtension) ResponseVerifyVoteExtension {
	return ResponseVerifyVoteExtension{
		Result: ResponseVerifyVoteExtension_ACCEPT,
	}
}

func (BaseApplication) Query(req RequestQuery) ResponseQuery {
	return ResponseQuery{Code: CodeTypeOK}
}

func (BaseApplication) InitChain(req RequestInitChain) ResponseInitChain {
	return ResponseInitChain{}
}

func (BaseApplication) ListSnapshots(req RequestListSnapshots) ResponseListSnapshots {
	return ResponseListSnapshots{}
}

func (BaseApplication) OfferSnapshot(req RequestOfferSnapshot) ResponseOfferSnapshot {
	return ResponseOfferSnapshot{}
}

func (BaseApplication) LoadSnapshotChunk(req RequestLoadSnapshotChunk) ResponseLoadSnapshotChunk {
	return ResponseLoadSnapshotChunk{}
}

func (BaseApplication) ApplySnapshotChunk(req RequestApplySnapshotChunk) ResponseApplySnapshotChunk {
	return ResponseApplySnapshotChunk{}
}

func (BaseApplication) PrepareProposal(req RequestPrepareProposal) ResponsePrepareProposal {
	return ResponsePrepareProposal{}
}

func (BaseApplication) ProcessProposal(req RequestProcessProposal) ResponseProcessProposal {
	return ResponseProcessProposal{}
}

func (BaseApplication) FinalizeBlock(req RequestFinalizeBlock) ResponseFinalizeBlock {
	txs := make([]*ResponseDeliverTx, len(req.Txs))
	for i := range req.Txs {
		txs[i] = &ResponseDeliverTx{Code: CodeTypeOK}
	}
	return ResponseFinalizeBlock{
		Txs: txs,
	}
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
	return &ResponseEcho{Message: req.Message}, nil
}

func (app *GRPCApplication) Flush(ctx context.Context, req *RequestFlush) (*ResponseFlush, error) {
	return &ResponseFlush{}, nil
}

func (app *GRPCApplication) Info(ctx context.Context, req *RequestInfo) (*ResponseInfo, error) {
	res := app.app.Info(*req)
	return &res, nil
}

func (app *GRPCApplication) CheckTx(ctx context.Context, req *RequestCheckTx) (*ResponseCheckTx, error) {
	res := app.app.CheckTx(*req)
	return &res, nil
}

func (app *GRPCApplication) Query(ctx context.Context, req *RequestQuery) (*ResponseQuery, error) {
	res := app.app.Query(*req)
	return &res, nil
}

func (app *GRPCApplication) Commit(ctx context.Context, req *RequestCommit) (*ResponseCommit, error) {
	res := app.app.Commit()
	return &res, nil
}

func (app *GRPCApplication) InitChain(ctx context.Context, req *RequestInitChain) (*ResponseInitChain, error) {
	res := app.app.InitChain(*req)
	return &res, nil
}

func (app *GRPCApplication) ListSnapshots(
	ctx context.Context, req *RequestListSnapshots) (*ResponseListSnapshots, error) {
	res := app.app.ListSnapshots(*req)
	return &res, nil
}

func (app *GRPCApplication) OfferSnapshot(
	ctx context.Context, req *RequestOfferSnapshot) (*ResponseOfferSnapshot, error) {
	res := app.app.OfferSnapshot(*req)
	return &res, nil
}

func (app *GRPCApplication) LoadSnapshotChunk(
	ctx context.Context, req *RequestLoadSnapshotChunk) (*ResponseLoadSnapshotChunk, error) {
	res := app.app.LoadSnapshotChunk(*req)
	return &res, nil
}

func (app *GRPCApplication) ApplySnapshotChunk(
	ctx context.Context, req *RequestApplySnapshotChunk) (*ResponseApplySnapshotChunk, error) {
	res := app.app.ApplySnapshotChunk(*req)
	return &res, nil
}

func (app *GRPCApplication) ExtendVote(
	ctx context.Context, req *RequestExtendVote) (*ResponseExtendVote, error) {
	res := app.app.ExtendVote(*req)
	return &res, nil
}

func (app *GRPCApplication) VerifyVoteExtension(
	ctx context.Context, req *RequestVerifyVoteExtension) (*ResponseVerifyVoteExtension, error) {
	res := app.app.VerifyVoteExtension(*req)
	return &res, nil
}

func (app *GRPCApplication) PrepareProposal(
	ctx context.Context, req *RequestPrepareProposal) (*ResponsePrepareProposal, error) {
	res := app.app.PrepareProposal(*req)
	return &res, nil
}

func (app *GRPCApplication) ProcessProposal(
	ctx context.Context, req *RequestProcessProposal) (*ResponseProcessProposal, error) {
	res := app.app.ProcessProposal(*req)
	return &res, nil
}

func (app *GRPCApplication) FinalizeBlock(
	ctx context.Context, req *RequestFinalizeBlock) (*ResponseFinalizeBlock, error) {
	res := app.app.FinalizeBlock(*req)
	return &res, nil
}
