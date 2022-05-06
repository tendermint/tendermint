package types

import "context"

//go:generate ../../scripts/mockery_generate.sh Application
// Application is an interface that enables any finite, deterministic state machine
// to be driven by a blockchain-based replication engine via the ABCI.
type Application interface {
	// Info/Query Connection
	Info(context.Context, *RequestInfo) (*ResponseInfo, error)    // Return application info
	Query(context.Context, *RequestQuery) (*ResponseQuery, error) // Query for state

	// Mempool Connection
	CheckTx(context.Context, *RequestCheckTx) (*ResponseCheckTx, error) // Validate a tx for the mempool

	// Consensus Connection
	InitChain(context.Context, *RequestInitChain) (*ResponseInitChain, error) // Initialize blockchain w validators/other info from TendermintCore
	PrepareProposal(context.Context, *RequestPrepareProposal) (*ResponsePrepareProposal, error)
	ProcessProposal(context.Context, *RequestProcessProposal) (*ResponseProcessProposal, error)
	// Commit the state and return the application Merkle root hash
	Commit(context.Context) (*ResponseCommit, error)
	// Create application specific vote extension
	ExtendVote(context.Context, *RequestExtendVote) (*ResponseExtendVote, error)
	// Verify application's vote extension data
	VerifyVoteExtension(context.Context, *RequestVerifyVoteExtension) (*ResponseVerifyVoteExtension, error)
	// Deliver the decided block with its txs to the Application
	FinalizeBlock(context.Context, *RequestFinalizeBlock) (*ResponseFinalizeBlock, error)

	// State Sync Connection
	ListSnapshots(context.Context, *RequestListSnapshots) (*ResponseListSnapshots, error)                // List available snapshots
	OfferSnapshot(context.Context, *RequestOfferSnapshot) (*ResponseOfferSnapshot, error)                // Offer a snapshot to the application
	LoadSnapshotChunk(context.Context, *RequestLoadSnapshotChunk) (*ResponseLoadSnapshotChunk, error)    // Load a snapshot chunk
	ApplySnapshotChunk(context.Context, *RequestApplySnapshotChunk) (*ResponseApplySnapshotChunk, error) // Apply a shapshot chunk
}

//-------------------------------------------------------
// BaseApplication is a base form of Application

var _ Application = (*BaseApplication)(nil)

type BaseApplication struct{}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(_ context.Context, req *RequestInfo) (*ResponseInfo, error) {
	return &ResponseInfo{}, nil
}

func (BaseApplication) CheckTx(_ context.Context, req *RequestCheckTx) (*ResponseCheckTx, error) {
	return &ResponseCheckTx{Code: CodeTypeOK}, nil
}

func (BaseApplication) Commit(_ context.Context) (*ResponseCommit, error) {
	return &ResponseCommit{}, nil
}

func (BaseApplication) ExtendVote(_ context.Context, req *RequestExtendVote) (*ResponseExtendVote, error) {
	return &ResponseExtendVote{}, nil
}

func (BaseApplication) VerifyVoteExtension(_ context.Context, req *RequestVerifyVoteExtension) (*ResponseVerifyVoteExtension, error) {
	return &ResponseVerifyVoteExtension{
		Status: ResponseVerifyVoteExtension_ACCEPT,
	}, nil
}

func (BaseApplication) Query(_ context.Context, req *RequestQuery) (*ResponseQuery, error) {
	return &ResponseQuery{Code: CodeTypeOK}, nil
}

func (BaseApplication) InitChain(_ context.Context, req *RequestInitChain) (*ResponseInitChain, error) {
	return &ResponseInitChain{}, nil
}

func (BaseApplication) ListSnapshots(_ context.Context, req *RequestListSnapshots) (*ResponseListSnapshots, error) {
	return &ResponseListSnapshots{}, nil
}

func (BaseApplication) OfferSnapshot(_ context.Context, req *RequestOfferSnapshot) (*ResponseOfferSnapshot, error) {
	return &ResponseOfferSnapshot{}, nil
}

func (BaseApplication) LoadSnapshotChunk(_ context.Context, _ *RequestLoadSnapshotChunk) (*ResponseLoadSnapshotChunk, error) {
	return &ResponseLoadSnapshotChunk{}, nil
}

func (BaseApplication) ApplySnapshotChunk(_ context.Context, req *RequestApplySnapshotChunk) (*ResponseApplySnapshotChunk, error) {
	return &ResponseApplySnapshotChunk{}, nil
}

func (BaseApplication) PrepareProposal(_ context.Context, req *RequestPrepareProposal) (*ResponsePrepareProposal, error) {
	trs := make([]*TxRecord, 0, len(req.Txs))
	var totalBytes int64
	for _, tx := range req.Txs {
		totalBytes += int64(len(tx))
		if totalBytes > req.MaxTxBytes {
			break
		}
		trs = append(trs, &TxRecord{
			Action: TxRecord_UNMODIFIED,
			Tx:     tx,
		})
	}
	return &ResponsePrepareProposal{TxRecords: trs}, nil
}

func (BaseApplication) ProcessProposal(_ context.Context, req *RequestProcessProposal) (*ResponseProcessProposal, error) {
	return &ResponseProcessProposal{Status: ResponseProcessProposal_ACCEPT}, nil
}

func (BaseApplication) FinalizeBlock(_ context.Context, req *RequestFinalizeBlock) (*ResponseFinalizeBlock, error) {
	txs := make([]*ExecTxResult, len(req.Txs))
	for i := range req.Txs {
		txs[i] = &ExecTxResult{Code: CodeTypeOK}
	}
	return &ResponseFinalizeBlock{
		TxResults: txs,
	}, nil
}
