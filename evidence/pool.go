package evpool

import (
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

// EvidencePool maintains a pool of valid evidence
// in an EvidenceStore.
type EvidencePool struct {
	config *cfg.EvidenceConfig
	logger log.Logger

	state         types.State
	evidenceStore *EvidenceStore

	evidenceChan chan types.Evidence
}

func NewEvidencePool(config *cfg.EvidenceConfig, evidenceStore *EvidenceStore, state types.State) *EvidencePool {
	evpool := &EvidencePool{
		config:        config,
		logger:        log.NewNopLogger(),
		evidenceStore: evidenceStore,
		state:         state,
		evidenceChan:  make(chan types.Evidence),
	}
	return evpool
}

// SetLogger sets the Logger.
func (evpool *EvidencePool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// EvidenceChan returns an unbuffered channel on which new evidence can be received.
func (evpool *EvidencePool) EvidenceChan() chan types.Evidence {
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

	// XXX: is this thread safe ?
	priority, err := evpool.state.VerifyEvidence(evidence)
	if err != nil {
		return err
	}

	added, err := evpool.evidenceStore.AddNewEvidence(evidence, priority)
	if err != nil {
		return err
	} else if !added {
		// evidence already known, just ignore
		return
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	evpool.evidenceChan <- evidence
	return nil
}

// MarkEvidenceAsCommitted marks all the evidence as committed.
func (evpool *EvidencePool) MarkEvidenceAsCommitted(evidence []types.Evidence) {
	for _, ev := range evidence {
		evpool.evidenceStore.MarkEvidenceAsCommitted(ev)
	}
}
