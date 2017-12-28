package evidence

import (
	"github.com/tendermint/tmlibs/log"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// EvidencePool maintains a pool of valid evidence
// in an EvidenceStore.
type EvidencePool struct {
	logger log.Logger

	evidenceStore *EvidenceStore

	state  sm.State
	params types.EvidenceParams

	// never close
	evidenceChan chan types.Evidence
}

func NewEvidencePool(params types.EvidenceParams, evidenceStore *EvidenceStore, state sm.State) *EvidencePool {
	evpool := &EvidencePool{
		state:         state,
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

	if err := sm.VerifyEvidence(evpool.state, evidence); err != nil {
		return err
	}

	var priority int64
	/* // Needs a db ...
	// TODO: if err is just that we cant find it cuz we pruned, ignore.
	// TODO: if its actually bad evidence, punish peer

	valset, err := LoadValidators(s.db, ev.Height())
	if err != nil {
		// XXX/TODO: what do we do if we can't load the valset?
		// eg. if we have pruned the state or height is too high?
		return err
	}
	if err := VerifyEvidenceValidator(valSet, ev); err != nil {
		return types.NewEvidenceInvalidErr(ev, err)
	}
	*/

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
