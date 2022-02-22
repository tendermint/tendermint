package mocks

import (
	types "github.com/tendermint/tendermint/abci/types"
)

type baseMock struct {
	base *types.BaseApplication
	*Application
}

func NewBaseMock() baseMock {
	return baseMock{
		base:        types.NewBaseApplication(),
		Application: new(Application),
	}
}

// Info/Query Connection
// Return application info
func (m baseMock) Info(input types.RequestInfo) types.ResponseInfo {
	var ret types.ResponseInfo
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.Info(input)
		}
	}()
	ret = m.Application.Info(input)
	return ret
}

func (m baseMock) Query(input types.RequestQuery) types.ResponseQuery {
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
func (m baseMock) CheckTx(input types.RequestCheckTx) types.ResponseCheckTx {
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
func (m baseMock) InitChain(input types.RequestInitChain) types.ResponseInitChain {
	var ret types.ResponseInitChain
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.InitChain(input)
		}
	}()
	ret = m.Application.InitChain(input)
	return ret
}

func (m baseMock) PrepareProposal(input types.RequestPrepareProposal) types.ResponsePrepareProposal {
	var ret types.ResponsePrepareProposal
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.PrepareProposal(input)
		}
	}()
	ret = m.Application.PrepareProposal(input)
	return ret
}

func (m baseMock) ProcessProposal(input types.RequestProcessProposal) types.ResponseProcessProposal {
	var r types.ResponseProcessProposal
	defer func() {
		if r := recover(); r != nil {
			r = m.base.ProcessProposal(input)
		}
	}()
	r = m.Application.ProcessProposal(input)
	return r
}

// Signals the beginning of a block
func (m baseMock) BeginBlock(input types.RequestBeginBlock) types.ResponseBeginBlock {
	var ret types.ResponseBeginBlock
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.BeginBlock(input)
		}
	}()
	ret = m.Application.BeginBlock(input)
	return ret
}

// Deliver a tx for full processing
func (m baseMock) DeliverTx(input types.RequestDeliverTx) types.ResponseDeliverTx {
	var ret types.ResponseDeliverTx
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.DeliverTx(input)
		}
	}()
	ret = m.Application.DeliverTx(input)
	return ret
}

// Signals the end of a block, returns changes to the validator set
func (m baseMock) EndBlock(input types.RequestEndBlock) types.ResponseEndBlock {
	var ret types.ResponseEndBlock
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.EndBlock(input)
		}
	}()
	ret = m.Application.EndBlock(input)
	return ret
}

// Commit the state and return the application Merkle root hash
func (m baseMock) Commit() types.ResponseCommit {
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
func (m baseMock) ExtendVote(input types.RequestExtendVote) types.ResponseExtendVote {
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
func (m baseMock) VerifyVoteExtension(input types.RequestVerifyVoteExtension) types.ResponseVerifyVoteExtension {
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
func (m baseMock) ListSnapshots(input types.RequestListSnapshots) types.ResponseListSnapshots {
	var ret types.ResponseListSnapshots
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ListSnapshots(input)
		}
	}()
	ret = m.Application.ListSnapshots(input)
	return ret
}

func (m baseMock) OfferSnapshot(input types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	var ret types.ResponseOfferSnapshot
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.OfferSnapshot(input)
		}
	}()
	ret = m.Application.OfferSnapshot(input)
	return ret
}

func (m baseMock) LoadSnapshotChunk(input types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	var ret types.ResponseLoadSnapshotChunk
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.LoadSnapshotChunk(input)
		}
	}()
	ret = m.Application.LoadSnapshotChunk(input)
	return ret
}

func (m baseMock) ApplySnapshotChunk(input types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	var ret types.ResponseApplySnapshotChunk
	defer func() {
		if r := recover(); r != nil {
			ret = m.base.ApplySnapshotChunk(input)
		}
	}()
	ret = m.Application.ApplySnapshotChunk(input)
	return ret
}
