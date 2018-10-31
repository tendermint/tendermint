package evidence

import (
	"fmt"
	"sync"

	clist "github.com/tendermint/tendermint/libs/clist"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// EvidencePool maintains a pool of valid evidence
// in an EvidenceStore.
type EvidencePool struct {
	logger log.Logger

	evidenceStore *EvidenceStore
	evidenceList  *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	stateDB dbm.DB

	// latest state
	mtx   sync.Mutex
	state sm.State
}

func NewEvidencePool(stateDB dbm.DB, evidenceStore *EvidenceStore) *EvidencePool {
	evpool := &EvidencePool{
		stateDB:       stateDB,
		state:         sm.LoadState(stateDB),
		logger:        log.NewNopLogger(),
		evidenceStore: evidenceStore,
		evidenceList:  clist.New(),
	}
	return evpool
}

func (evpool *EvidencePool) EvidenceFront() *clist.CElement {
	return evpool.evidenceList.Front()
}

func (evpool *EvidencePool) EvidenceWaitChan() <-chan struct{} {
	return evpool.evidenceList.WaitChan()
}

// SetLogger sets the Logger.
func (evpool *EvidencePool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// PriorityEvidence returns the priority evidence.
func (evpool *EvidencePool) PriorityEvidence() []types.Evidence {
	return evpool.evidenceStore.PriorityEvidence()
}

// PendingEvidence returns uncommitted evidence up to maxBytes.
// If maxBytes is -1, all evidence is returned.
func (evpool *EvidencePool) PendingEvidence(maxBytes int64) []types.Evidence {
	return evpool.evidenceStore.PendingEvidence(maxBytes)
}

// State returns the current state of the evpool.
func (evpool *EvidencePool) State() sm.State {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return evpool.state
}

// Update loads the latest
func (evpool *EvidencePool) Update(block *types.Block, state sm.State) {

	// sanity check
	if state.LastBlockHeight != block.Height {
		panic(fmt.Sprintf("Failed EvidencePool.Update sanity check: got state.Height=%d with block.Height=%d", state.LastBlockHeight, block.Height))
	}

	// update the state
	evpool.mtx.Lock()
	evpool.state = state
	evpool.mtx.Unlock()

	// remove evidence from pending and mark committed
	evpool.MarkEvidenceAsCommitted(block.Height, block.Evidence.Evidence)
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *EvidencePool) AddEvidence(evidence types.Evidence) (err error) {

	// TODO: check if we already have evidence for this
	// validator at this height so we dont get spammed

	if err := sm.VerifyEvidence(evpool.stateDB, evpool.State(), evidence); err != nil {
		return err
	}

	// fetch the validator and return its voting power as its priority
	// TODO: something better ?
	valset, _ := sm.LoadValidators(evpool.stateDB, evidence.Height())
	_, val := valset.GetByAddress(evidence.Address())
	priority := val.VotingPower

	added := evpool.evidenceStore.AddNewEvidence(evidence, priority)
	if !added {
		// evidence already known, just ignore
		return
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	// add evidence to clist
	evpool.evidenceList.PushBack(evidence)

	return nil
}

// MarkEvidenceAsCommitted marks all the evidence as committed and removes it from the queue.
func (evpool *EvidencePool) MarkEvidenceAsCommitted(height int64, evidence []types.Evidence) {
	// make a map of committed evidence to remove from the clist
	blockEvidenceMap := make(map[string]struct{})
	for _, ev := range evidence {
		evpool.evidenceStore.MarkEvidenceAsCommitted(ev)
		blockEvidenceMap[evMapKey(ev)] = struct{}{}
	}

	// remove committed evidence from the clist
	maxAge := evpool.State().ConsensusParams.Evidence.MaxAge
	evpool.removeEvidence(height, maxAge, blockEvidenceMap)

}

func (evpool *EvidencePool) removeEvidence(height, maxAge int64, blockEvidenceMap map[string]struct{}) {
	for e := evpool.evidenceList.Front(); e != nil; e = e.Next() {
		ev := e.Value.(types.Evidence)

		// Remove the evidence if it's already in a block
		// or if it's now too old.
		if _, ok := blockEvidenceMap[evMapKey(ev)]; ok ||
			ev.Height() < height-maxAge {

			// remove from clist
			evpool.evidenceList.Remove(e)
			e.DetachPrev()
		}
	}
}

func evMapKey(ev types.Evidence) string {
	return string(ev.Hash())
}
