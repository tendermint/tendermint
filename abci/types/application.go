package types

import "context"

//go:generate ../../scripts/mockery_generate.sh Application
// Application is an interface that enables any finite, deterministic state machine
// to be driven by a blockchain-based replication engine via the ABCI.
// All methods take a RequestXxx argument and return a ResponseXxx argument,
// except CheckTx/DeliverTx, which take `tx []byte`, and `Commit`, which takes nothing.
type Application interface {
	// Info/Query Connection
	Info(context.Context, RequestInfo) ResponseInfo    // Return application info
	Query(context.Context, RequestQuery) ResponseQuery // Query for state

	// Mempool Connection
	CheckTx(context.Context, RequestCheckTx) ResponseCheckTx // Validate a tx for the mempool

	// Consensus Connection
	InitChain(context.Context, RequestInitChain) ResponseInitChain // Initialize blockchain w validators/other info from TendermintCore
	PrepareProposal(context.Context, RequestPrepareProposal) ResponsePrepareProposal
	ProcessProposal(context.Context, RequestProcessProposal) ResponseProcessProposal
	// Commit the state and return the application Merkle root hash
	Commit(context.Context) ResponseCommit
	// Create application specific vote extension
	ExtendVote(context.Context, RequestExtendVote) ResponseExtendVote
	// Verify application's vote extension data
	VerifyVoteExtension(context.Context, RequestVerifyVoteExtension) ResponseVerifyVoteExtension
	// Deliver the decided block with its txs to the Application
	FinalizeBlock(context.Context, RequestFinalizeBlock) ResponseFinalizeBlock

	// State Sync Connection
	ListSnapshots(context.Context, RequestListSnapshots) ResponseListSnapshots                // List available snapshots
	OfferSnapshot(context.Context, RequestOfferSnapshot) ResponseOfferSnapshot                // Offer a snapshot to the application
	LoadSnapshotChunk(context.Context, RequestLoadSnapshotChunk) ResponseLoadSnapshotChunk    // Load a snapshot chunk
	ApplySnapshotChunk(context.Context, RequestApplySnapshotChunk) ResponseApplySnapshotChunk // Apply a shapshot chunk
}

//-------------------------------------------------------
// BaseApplication is a base form of Application

var _ Application = (*BaseApplication)(nil)

type BaseApplication struct{}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(_ context.Context, req RequestInfo) ResponseInfo {
	return ResponseInfo{}
}

func (BaseApplication) CheckTx(_ context.Context, req RequestCheckTx) ResponseCheckTx {
	return ResponseCheckTx{Code: CodeTypeOK}
}

func (BaseApplication) Commit(_ context.Context) ResponseCommit {
	return ResponseCommit{}
}

func (BaseApplication) ExtendVote(_ context.Context, req RequestExtendVote) ResponseExtendVote {
	return ResponseExtendVote{}
}

func (BaseApplication) VerifyVoteExtension(_ context.Context, req RequestVerifyVoteExtension) ResponseVerifyVoteExtension {
	return ResponseVerifyVoteExtension{
		Status: ResponseVerifyVoteExtension_ACCEPT,
	}
}

func (BaseApplication) Query(_ context.Context, req RequestQuery) ResponseQuery {
	return ResponseQuery{Code: CodeTypeOK}
}

func (BaseApplication) InitChain(_ context.Context, req RequestInitChain) ResponseInitChain {
	return ResponseInitChain{}
}

func (BaseApplication) ListSnapshots(_ context.Context, req RequestListSnapshots) ResponseListSnapshots {
	return ResponseListSnapshots{}
}

func (BaseApplication) OfferSnapshot(_ context.Context, req RequestOfferSnapshot) ResponseOfferSnapshot {
	return ResponseOfferSnapshot{}
}

func (BaseApplication) LoadSnapshotChunk(_ context.Context, _ RequestLoadSnapshotChunk) ResponseLoadSnapshotChunk {
	return ResponseLoadSnapshotChunk{}
}

func (BaseApplication) ApplySnapshotChunk(_ context.Context, req RequestApplySnapshotChunk) ResponseApplySnapshotChunk {
	return ResponseApplySnapshotChunk{}
}

func (BaseApplication) PrepareProposal(_ context.Context, req RequestPrepareProposal) ResponsePrepareProposal {
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
	return ResponsePrepareProposal{TxRecords: trs}
}

func (BaseApplication) ProcessProposal(_ context.Context, req RequestProcessProposal) ResponseProcessProposal {
	return ResponseProcessProposal{Status: ResponseProcessProposal_ACCEPT}
}

func (BaseApplication) FinalizeBlock(_ context.Context, req RequestFinalizeBlock) ResponseFinalizeBlock {
	txs := make([]*ExecTxResult, len(req.Txs))
	for i := range req.Txs {
		txs[i] = &ExecTxResult{Code: CodeTypeOK}
	}
	return ResponseFinalizeBlock{
		TxResults: txs,
	}
}
