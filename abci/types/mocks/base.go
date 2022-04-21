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
func (m BaseMock) Info(ctx context.Context, input types.RequestInfo) (ret *types.ResponseInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.Info(ctx, input)
		}
	}()
	ret, err = m.Application.Info(ctx, input)
	return
}

func (m BaseMock) Query(ctx context.Context, input types.RequestQuery) (ret *types.ResponseQuery, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.Query(ctx, input)
		}
	}()
	ret, err = m.Application.Query(ctx, input)
	return
}

// Mempool Connection
// Validate a tx for the mempool
func (m BaseMock) CheckTx(ctx context.Context, input types.RequestCheckTx) (ret *types.ResponseCheckTx, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.CheckTx(ctx, input)
		}
	}()
	ret, err = m.Application.CheckTx(ctx, input)
	return
}

// Consensus Connection
// Initialize blockchain w validators/other info from TendermintCore
func (m BaseMock) InitChain(ctx context.Context, input types.RequestInitChain) (ret *types.ResponseInitChain, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.InitChain(ctx, input)
		}
	}()
	ret, err = m.Application.InitChain(ctx, input)
	return
}

func (m BaseMock) PrepareProposal(ctx context.Context, input types.RequestPrepareProposal) (ret *types.ResponsePrepareProposal, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.PrepareProposal(ctx, input)
		}
	}()
	ret, err = m.Application.PrepareProposal(ctx, input)
	return
}

func (m BaseMock) ProcessProposal(ctx context.Context, input types.RequestProcessProposal) (ret *types.ResponseProcessProposal, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.ProcessProposal(ctx, input)
		}
	}()
	ret, err = m.Application.ProcessProposal(ctx, input)
	return
}

// Commit the state and return the application Merkle root hash
func (m BaseMock) Commit(ctx context.Context) (ret *types.ResponseCommit, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.Commit(ctx)
		}
	}()
	ret, err = m.Application.Commit(ctx)
	return
}

// Create application specific vote extension
func (m BaseMock) ExtendVote(ctx context.Context, input types.RequestExtendVote) (ret *types.ResponseExtendVote, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.ExtendVote(ctx, input)
		}
	}()
	ret, err = m.Application.ExtendVote(ctx, input)
	return
}

// Verify application's vote extension data
func (m BaseMock) VerifyVoteExtension(ctx context.Context, input types.RequestVerifyVoteExtension) (ret *types.ResponseVerifyVoteExtension, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.VerifyVoteExtension(ctx, input)
		}
	}()
	ret, err = m.Application.VerifyVoteExtension(ctx, input)
	return
}

// State Sync Connection
// List available snapshots
func (m BaseMock) ListSnapshots(ctx context.Context, input types.RequestListSnapshots) (ret *types.ResponseListSnapshots, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.ListSnapshots(ctx, input)
		}
	}()
	ret, err = m.Application.ListSnapshots(ctx, input)
	return
}

func (m BaseMock) OfferSnapshot(ctx context.Context, input types.RequestOfferSnapshot) (ret *types.ResponseOfferSnapshot, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.OfferSnapshot(ctx, input)
		}
	}()
	ret, err = m.Application.OfferSnapshot(ctx, input)
	return
}

func (m BaseMock) LoadSnapshotChunk(ctx context.Context, input types.RequestLoadSnapshotChunk) (ret *types.ResponseLoadSnapshotChunk, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.LoadSnapshotChunk(ctx, input)
		}
	}()
	ret, err = m.Application.LoadSnapshotChunk(ctx, input)
	return
}

func (m BaseMock) ApplySnapshotChunk(ctx context.Context, input types.RequestApplySnapshotChunk) (ret *types.ResponseApplySnapshotChunk, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.ApplySnapshotChunk(ctx, input)
		}
	}()
	ret, err = m.Application.ApplySnapshotChunk(ctx, input)
	return
}

func (m BaseMock) FinalizeBlock(ctx context.Context, input types.RequestFinalizeBlock) (ret *types.ResponseFinalizeBlock, err error) {
	defer func() {
		if r := recover(); r != nil {
			ret, err = m.base.FinalizeBlock(ctx, input)
		}
	}()
	ret, err = m.Application.FinalizeBlock(ctx, input)
	return
}
