package mocks

import (
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
func (m BaseMock) Info(input types.RequestInfo) types.ResponseInfo {
	var ret types.ResponseInfo
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Info(input)
		}
	}()
	ret = m.Application.Info(input)
	return ret
}

func (m BaseMock) Query(input types.RequestQuery) types.ResponseQuery {
	var ret types.ResponseQuery
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Query(input)
		}
	}()
	ret = m.Application.Query(input)
	return ret
}

// Mempool Connection
// Validate a tx for the mempool
func (m BaseMock) CheckTx(input types.RequestCheckTx) types.ResponseCheckTx {
	var ret types.ResponseCheckTx
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.CheckTx(input)
		}
	}()
	ret = m.Application.CheckTx(input)
	return ret
}

// Consensus Connection
// Initialize blockchain w validators/other info from TendermintCore
func (m BaseMock) InitChain(input types.RequestInitChain) types.ResponseInitChain {
	var ret types.ResponseInitChain
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.InitChain(input)
		}
	}()
	ret = m.Application.InitChain(input)
	return ret
}

func (m BaseMock) PrepareProposal(input types.RequestPrepareProposal) types.ResponsePrepareProposal {
	var ret types.ResponsePrepareProposal
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.PrepareProposal(input)
		}
	}()
	ret = m.Application.PrepareProposal(input)
	return ret
}

func (m BaseMock) ProcessProposal(input types.RequestProcessProposal) types.ResponseProcessProposal {
	var ret types.ResponseProcessProposal
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ProcessProposal(input)
		}
	}()
	ret = m.Application.ProcessProposal(input)
	return ret
}

// Commit the state and return the application Merkle root hash
func (m BaseMock) Commit() types.ResponseCommit {
	var ret types.ResponseCommit
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Commit()
		}
	}()
	ret = m.Application.Commit()
	return ret
}

// Create application specific vote extension
func (m BaseMock) ExtendVote(input types.RequestExtendVote) types.ResponseExtendVote {
	var ret types.ResponseExtendVote
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ExtendVote(input)
		}
	}()
	ret = m.Application.ExtendVote(input)
	return ret
}

// Verify application's vote extension data
func (m BaseMock) VerifyVoteExtension(input types.RequestVerifyVoteExtension) types.ResponseVerifyVoteExtension {
	var ret types.ResponseVerifyVoteExtension
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.VerifyVoteExtension(input)
		}
	}()
	ret = m.Application.VerifyVoteExtension(input)
	return ret
}

// State Sync Connection
// List available snapshots
func (m BaseMock) ListSnapshots(input types.RequestListSnapshots) types.ResponseListSnapshots {
	var ret types.ResponseListSnapshots
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ListSnapshots(input)
		}
	}()
	ret = m.Application.ListSnapshots(input)
	return ret
}

func (m BaseMock) OfferSnapshot(input types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	var ret types.ResponseOfferSnapshot
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.OfferSnapshot(input)
		}
	}()
	ret = m.Application.OfferSnapshot(input)
	return ret
}

func (m BaseMock) LoadSnapshotChunk(input types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	var ret types.ResponseLoadSnapshotChunk
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.LoadSnapshotChunk(input)
		}
	}()
	ret = m.Application.LoadSnapshotChunk(input)
	return ret
}

func (m BaseMock) ApplySnapshotChunk(input types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	var ret types.ResponseApplySnapshotChunk
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ApplySnapshotChunk(input)
		}
	}()
	ret = m.Application.ApplySnapshotChunk(input)
	return ret
}

func (m BaseMock) FinalizeBlock(input types.RequestFinalizeBlock) types.ResponseFinalizeBlock {
	var ret types.ResponseFinalizeBlock
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.FinalizeBlock(input)
		}
	}()
	ret = m.Application.FinalizeBlock(input)
	return ret
}
