package mocks

import (
	context "context"

	types "github.com/tendermint/tendermint/abci/types"
)

// BaseMock provides a wrapper around the generated Application mock and a BaseApplication.
// BaseMock first tries to use the mock's implementation of the method.
// If no functionality was provided for the mock by the user, BaseMock dispatches
// to the BaseApplication and uses its functionality.
// BaseMock allows users to provide mocked functionality for only the methods that matter
// for their test while avoiding a panic if the code calls Application methods that are
// not relevant to the test.
type BaseMock struct {
	base *types.BaseApplication
	*Application
}

func NewBaseMock() BaseMock {
	return BaseMock{
		base:        types.NewBaseApplication(),
		Application: new(Application),
	}
}

// Info/Query Connection
// Return application info
func (m BaseMock) Info(ctx context.Context, input types.RequestInfo) (ret types.ResponseInfo) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Info(ctx, input)
		}
	}()
	ret = m.Application.Info(ctx, input)
	return ret
}

func (m BaseMock) Query(ctx context.Context, input types.RequestQuery) (ret types.ResponseQuery) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Query(ctx, input)
		}
	}()
	ret = m.Application.Query(ctx, input)
	return ret
}

// Mempool Connection
// Validate a tx for the mempool
func (m BaseMock) CheckTx(ctx context.Context, input types.RequestCheckTx) (ret types.ResponseCheckTx) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.CheckTx(ctx, input)
		}
	}()
	ret = m.Application.CheckTx(ctx, input)
	return ret
}

// Consensus Connection
// Initialize blockchain w validators/other info from TendermintCore
func (m BaseMock) InitChain(ctx context.Context, input types.RequestInitChain) (ret types.ResponseInitChain) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.InitChain(ctx, input)
		}
	}()
	ret = m.Application.InitChain(ctx, input)
	return ret
}

func (m BaseMock) PrepareProposal(ctx context.Context, input types.RequestPrepareProposal) (ret types.ResponsePrepareProposal) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.PrepareProposal(ctx, input)
		}
	}()
	ret = m.Application.PrepareProposal(ctx, input)
	return ret
}

func (m BaseMock) ProcessProposal(ctx context.Context, input types.RequestProcessProposal) (ret types.ResponseProcessProposal) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ProcessProposal(ctx, input)
		}
	}()
	ret = m.Application.ProcessProposal(ctx, input)
	return ret
}

// Commit the state and return the application Merkle root hash
func (m BaseMock) Commit(ctx context.Context) (ret types.ResponseCommit) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Commit(ctx)
		}
	}()
	ret = m.Application.Commit(ctx)
	return ret
}

// Create application specific vote extension
func (m BaseMock) ExtendVote(ctx context.Context, input types.RequestExtendVote) (ret types.ResponseExtendVote) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ExtendVote(ctx, input)
		}
	}()
	ret = m.Application.ExtendVote(ctx, input)
	return ret
}

// Verify application's vote extension data
func (m BaseMock) VerifyVoteExtension(ctx context.Context, input types.RequestVerifyVoteExtension) (ret types.ResponseVerifyVoteExtension) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.VerifyVoteExtension(ctx, input)
		}
	}()
	ret = m.Application.VerifyVoteExtension(ctx, input)
	return ret
}

// State Sync Connection
// List available snapshots
func (m BaseMock) ListSnapshots(ctx context.Context, input types.RequestListSnapshots) (ret types.ResponseListSnapshots) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ListSnapshots(ctx, input)
		}
	}()
	ret = m.Application.ListSnapshots(ctx, input)
	return ret
}

func (m BaseMock) OfferSnapshot(ctx context.Context, input types.RequestOfferSnapshot) (ret types.ResponseOfferSnapshot) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.OfferSnapshot(ctx, input)
		}
	}()
	ret = m.Application.OfferSnapshot(ctx, input)
	return ret
}

func (m BaseMock) LoadSnapshotChunk(ctx context.Context, input types.RequestLoadSnapshotChunk) (ret types.ResponseLoadSnapshotChunk) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.LoadSnapshotChunk(ctx, input)
		}
	}()
	ret = m.Application.LoadSnapshotChunk(ctx, input)
	return ret
}

func (m BaseMock) ApplySnapshotChunk(ctx context.Context, input types.RequestApplySnapshotChunk) (ret types.ResponseApplySnapshotChunk) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ApplySnapshotChunk(ctx, input)
		}
	}()
	ret = m.Application.ApplySnapshotChunk(ctx, input)
	return ret
}

func (m BaseMock) FinalizeBlock(ctx context.Context, input types.RequestFinalizeBlock) (ret types.ResponseFinalizeBlock) {
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.FinalizeBlock(ctx, input)
		}
	}()
	ret = m.Application.FinalizeBlock(ctx, input)
	return ret
}
