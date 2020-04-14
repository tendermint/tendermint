package evidence

import (
	"fmt"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Pool maintains a pool of valid evidence
// in an Store.
type Pool struct {
	logger log.Logger

	store        *Store
	evidenceList *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	stateDB dbm.DB

	// latest state
	mtx   sync.Mutex
	state sm.State
}

func NewPool(stateDB, evidenceDB dbm.DB) *Pool {
	store := NewStore(evidenceDB)
	evpool := &Pool{
		stateDB:      stateDB,
		state:        sm.LoadState(stateDB),
		logger:       log.NewNopLogger(),
		store:        store,
		evidenceList: clist.New(),
	}
	return evpool
}

func (evpool *Pool) EvidenceFront() *clist.CElement {
	return evpool.evidenceList.Front()
}

func (evpool *Pool) EvidenceWaitChan() <-chan struct{} {
	return evpool.evidenceList.WaitChan()
}

// SetLogger sets the Logger.
func (evpool *Pool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// PriorityEvidence returns the priority evidence.
func (evpool *Pool) PriorityEvidence() []types.Evidence {
	return evpool.store.PriorityEvidence()
}

// PendingEvidence returns up to maxNum uncommitted evidence.
// If maxNum is -1, all evidence is returned.
func (evpool *Pool) PendingEvidence(maxNum int64) []types.Evidence {
	return evpool.store.PendingEvidence(maxNum)
}

// State returns the current state of the evpool.
func (evpool *Pool) State() sm.State {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return evpool.state
}

// Update loads the latest
func (evpool *Pool) Update(block *types.Block, state sm.State) {

	// sanity check
	if state.LastBlockHeight != block.Height {
		panic(
			fmt.Sprintf("Failed EvidencePool.Update sanity check: got state.Height=%d with block.Height=%d",
				state.LastBlockHeight,
				block.Height,
			),
		)
	}

	// update the state
	evpool.mtx.Lock()
	evpool.state = state
	evpool.mtx.Unlock()

	// remove evidence from pending and mark committed
	evpool.MarkEvidenceAsCommitted(block.Height, block.Time, block.Evidence.Evidence)
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *Pool) AddEvidence(evidence types.Evidence) error {

	// check if evidence is already stored
	if evpool.store.Has(evidence) {
		return ErrEvidenceAlreadyStored{}
	}

	if err := sm.VerifyEvidence(evpool.stateDB, evpool.State(), evidence); err != nil {
		return ErrInvalidEvidence{err}
	}

	// fetch the validator and return its voting power as its priority
	// TODO: something better ?
	valset, err := sm.LoadValidators(evpool.stateDB, evidence.Height())
	if err != nil {
		return err
	}
	_, val := valset.GetByAddress(evidence.Address())
	priority := val.VotingPower

	_, err = evpool.store.AddNewEvidence(evidence, priority)
	if err != nil {
		return err
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	// add evidence to clist
	evpool.evidenceList.PushBack(evidence)

	return nil
}

// MarkEvidenceAsCommitted marks all the evidence as committed and removes it from the queue.
func (evpool *Pool) MarkEvidenceAsCommitted(height int64, lastBlockTime time.Time, evidence []types.Evidence) {
	// make a map of committed evidence to remove from the clist
	blockEvidenceMap := make(map[string]struct{})
	for _, ev := range evidence {
		evpool.store.MarkEvidenceAsCommitted(ev)
		blockEvidenceMap[evMapKey(ev)] = struct{}{}
	}

	// remove committed evidence from the clist
	evidenceParams := evpool.State().ConsensusParams.Evidence
	evpool.removeEvidence(height, lastBlockTime, evidenceParams, blockEvidenceMap)
}

// IsCommitted returns true if we have already seen this exact evidence and it is already marked as committed.
func (evpool *Pool) IsCommitted(evidence types.Evidence) bool {
	ei := evpool.store.getInfo(evidence)
	return ei.Evidence != nil && ei.Committed
}

func (evpool *Pool) removeEvidence(
	height int64,
	lastBlockTime time.Time,
	params types.EvidenceParams,
	blockEvidenceMap map[string]struct{}) {

	for e := evpool.evidenceList.Front(); e != nil; e = e.Next() {
		var (
			ev           = e.Value.(types.Evidence)
			ageDuration  = lastBlockTime.Sub(ev.Time())
			ageNumBlocks = height - ev.Height()
		)

		// Remove the evidence if it's already in a block or if it's now too old.
		if _, ok := blockEvidenceMap[evMapKey(ev)]; ok ||
			(ageDuration > params.MaxAgeDuration && ageNumBlocks > params.MaxAgeNumBlocks) {
			// remove from clist
			evpool.evidenceList.Remove(e)
			e.DetachPrev()
		}
	}
}

func evMapKey(ev types.Evidence) string {
	return string(ev.Hash())
}
