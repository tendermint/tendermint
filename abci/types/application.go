package types

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
		Status: ResponseVerifyVoteExtension_ACCEPT,
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

func (BaseApplication) ProcessProposal(req RequestProcessProposal) ResponseProcessProposal {
	return ResponseProcessProposal{Status: ResponseProcessProposal_ACCEPT}
}

func (BaseApplication) FinalizeBlock(req RequestFinalizeBlock) ResponseFinalizeBlock {
	txs := make([]*ExecTxResult, len(req.Txs))
	for i := range req.Txs {
		txs[i] = &ExecTxResult{Code: CodeTypeOK}
	}
	return ResponseFinalizeBlock{
		TxResults: txs,
	}
}
