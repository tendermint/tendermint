package mock

import (
	"time"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// A quick and easy in-memory mock implementation of the evidence pool configured with both the standard exposed
// functions and extra helper function to set up, operate and ultimately imitate the evidence pool in a testing
// environment. Note: this mock does no validation. Do not test with large amounts of evidence
type EvidencePool struct {
	PendingEvidenceList   []types.Evidence
	CommittedEvidenceList []types.Evidence
	ExpirationAgeTime     time.Duration
	ExpirationAgeBlock    int64
	BlockHeight           int64
	BlockTime             time.Time
}

// ------------------------ INSTANTIATION METHODS --------------------------------------

func NewEvidencePool(height, expiryHeight int64, currentTime time.Time, expiryTime time.Duration) *EvidencePool {
	return &EvidencePool{
		[]types.Evidence{},
		[]types.Evidence{},
		expiryTime,
		expiryHeight,
		height,
		currentTime,
	}
}

func NewDefaultEvidencePool() *EvidencePool {
	return NewEvidencePool(1, 1, time.Now(), time.Millisecond)
}

// ----------------------- EVIDENCE POOL PUBLIC METHODS --------------------------------

func (p *EvidencePool) PendingEvidence(maxNum int64) []types.Evidence {
	if maxNum == -1 || maxNum >= int64(len(p.PendingEvidenceList)) {
		return p.PendingEvidenceList
	}
	return p.PendingEvidenceList[:maxNum]
}

func (p *EvidencePool) AddEvidence(evidence types.Evidence) error {
	p.PendingEvidenceList = append(p.PendingEvidenceList, evidence)
	return nil
}

func (p *EvidencePool) Update(block *types.Block, state sm.State) {
	p.BlockHeight = block.Height
	p.BlockTime = block.Time
	p.MarkEvidenceAsCommitted(block.Height, block.Time, block.Evidence.Evidence)
	p.RemoveExpiredEvidence()
}

func (p *EvidencePool) MarkEvidenceAsCommitted(height int64, lastBlockTime time.Time, evidence []types.Evidence) {
	for _, ev := range evidence {
		if p.IsPending(ev) {
			p.RemovePendingEvidence(ev)
		}
		p.CommittedEvidenceList = append(p.CommittedEvidenceList, ev)
	}
}

func (p *EvidencePool) IsPending(evidence types.Evidence) bool {
	for _, ev := range p.PendingEvidenceList {
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
}

func (p *EvidencePool) IsCommitted(evidence types.Evidence) bool {
	for _, ev := range p.CommittedEvidenceList {
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
}

func (p *EvidencePool) IsExpired(evidence types.Evidence) bool {
	return evidence.Height()+p.ExpirationAgeBlock < p.BlockHeight &&
		evidence.Time().Add(p.ExpirationAgeTime).Before(p.BlockTime)
}

// ------------------------------- HELPER METHODS --------------------------------------

func (p *EvidencePool) RemovePendingEvidence(evidence types.Evidence) {
	for idx, ev := range p.PendingEvidenceList {
		if ev.Equal(evidence) {
			p.PendingEvidenceList[idx] = p.PendingEvidenceList[len(p.PendingEvidenceList)-1]
			p.PendingEvidenceList = p.PendingEvidenceList[:len(p.PendingEvidenceList)-1]
			return
		}
	}
}

func (p *EvidencePool) RemoveExpiredEvidence() {
	for _, evidence := range p.PendingEvidenceList {
		if p.IsExpired(evidence) {
			p.RemovePendingEvidence(evidence)
		}
	}
}

func (p *EvidencePool) AddMockEvidence(height int64, address []byte) types.Evidence {
	mock := types.MockEvidence{
		EvidenceHeight:  height,
		EvidenceTime:    p.BlockTime,
		EvidenceAddress: address,
	}
	_ = p.AddEvidence(mock)
	return mock
}

func (p *EvidencePool) CommitEvidence(evidence types.Evidence) {
	p.MarkEvidenceAsCommitted(evidence.Height(), evidence.Time(), []types.Evidence{evidence})
}
