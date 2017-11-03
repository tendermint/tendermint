package evpool

import (
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/types"
)

const cacheSize = 100000

// EvidencePool maintains a set of valid uncommitted evidence.
type EvidencePool struct {
	config *EvidencePoolConfig
	logger log.Logger

	evidenceStore   *EvidenceStore
	newEvidenceChan chan types.Evidence
}

type EvidencePoolConfig struct {
}

func NewEvidencePool(config *EvidencePoolConfig, evidenceStore *EvidenceStore) *EvidencePool {
	evpool := &EvidencePool{
		config:          config,
		logger:          log.NewNopLogger(),
		evidenceStore:   evidenceStore,
		newEvidenceChan: make(chan types.Evidence),
	}
	return evpool
}

// SetLogger sets the Logger.
func (evpool *EvidencePool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// NewEvidenceChan returns a channel on which new evidence is sent.
func (evpool *EvidencePool) NewEvidenceChan() chan types.Evidence {
	return evpool.newEvidenceChan
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
func (evpool *EvidencePool) AddEvidence(evidence types.Evidence) (err error) {
	added, err := evpool.evidenceStore.AddNewEvidence(evidence)
	if err != nil {
		return err
	} else if !added {
		// evidence already known, just ignore
		return
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	evpool.newEvidenceChan <- evidence
	return nil
}

// Update informs the evpool that the given evidence was committed and can be discarded.
// NOTE: this should be called *after* block is committed by consensus.
func (evpool *EvidencePool) Update(height int, evidence []types.Evidence) {

	// First, create a lookup map of new committed evidence

	evMap := make(map[string]struct{})
	for _, ev := range evidence {
		evpool.evidenceStore.MarkEvidenceAsCommitted(ev)
		evMap[string(ev.Hash())] = struct{}{}
	}

	// Remove evidence that is already committed .
	goodEvidence := evpool.filterEvidence(evMap)
	_ = goodEvidence

}

func (evpool *EvidencePool) filterEvidence(blockEvidenceMap map[string]struct{}) []types.Evidence {
	// TODO:
	return nil
}
