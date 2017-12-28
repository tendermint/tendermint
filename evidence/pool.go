package evidence

import (
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/types"
)

// EvidencePool maintains a pool of valid evidence
// in an EvidenceStore.
type EvidencePool struct {
	logger log.Logger

	evidenceStore *EvidenceStore

	chainID         string
	lastBlockHeight int64
	params          types.EvidenceParams

	// never close
	evidenceChan chan types.Evidence
}

func NewEvidencePool(params types.EvidenceParams, evidenceStore *EvidenceStore) *EvidencePool {
	evpool := &EvidencePool{
		params:        params,
		logger:        log.NewNopLogger(),
		evidenceStore: evidenceStore,
		// state:         *state,
		evidenceChan: make(chan types.Evidence),
	}
	return evpool
}

// SetLogger sets the Logger.
func (evpool *EvidencePool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// EvidenceChan returns an unbuffered channel on which new evidence can be received.
func (evpool *EvidencePool) EvidenceChan() <-chan types.Evidence {
	return evpool.evidenceChan
}

// PriorityEvidence returns the priority evidence.
func (evpool *EvidencePool) PriorityEvidence() []types.Evidence {
	return evpool.evidenceStore.PriorityEvidence()
}

// PendingEvidence returns all uncommitted evidence.
func (evpool *EvidencePool) PendingEvidence() []types.Evidence {
	return evpool.evidenceStore.PendingEvidence()
}

// AddEvidence checks the evidence is valid and adds it to the pool.
// Blocks on the EvidenceChan.
func (evpool *EvidencePool) AddEvidence(evidence types.Evidence) (err error) {
	// TODO: check if we already have evidence for this
	// validator at this height so we dont get spammed

	// TODO
	var priority int64
	/*
		priority, err := sm.VerifyEvidence(evpool.state, evidence)
		if err != nil {
			// TODO: if err is just that we cant find it cuz we pruned, ignore.
			// TODO: if its actually bad evidence, punish peer
			return err
		}*/

	added := evpool.evidenceStore.AddNewEvidence(evidence, priority)
	if !added {
		// evidence already known, just ignore
		return
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	// never closes. always safe to send on
	evpool.evidenceChan <- evidence
	return nil
}

// MarkEvidenceAsCommitted marks all the evidence as committed.
func (evpool *EvidencePool) MarkEvidenceAsCommitted(evidence []types.Evidence) {
	for _, ev := range evidence {
		evpool.evidenceStore.MarkEvidenceAsCommitted(ev)
	}
}
