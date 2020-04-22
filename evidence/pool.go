package evidence

import (
	"fmt"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

// Pool maintains a pool of valid evidence in an Store.
type Pool struct {
	logger log.Logger

	store        *Store
	evidenceList *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	stateDB    dbm.DB
	blockStore *store.BlockStore

	// a map of active validators and respective last heights validator is active
	// if it was in validator set after EvidenceParams.MaxAgeNumBlocks or
	// currently is (ie. [MaxAgeNumBlocks, CurrentHeight])
	// In simple words, it means it's still bonded -> therefore slashable.
	valToLastHeight valToLastHeightMap

	// latest state
	mtx   sync.Mutex
	state sm.State
}

// Validator.Address -> Last height it was in validator set
type valToLastHeightMap map[string]int64

func NewPool(stateDB, evidenceDB dbm.DB, blockStore *store.BlockStore) (*Pool, error) {
	var (
		store = NewStore(evidenceDB)
		state = sm.LoadState(stateDB)
	)

	valToLastHeight, err := buildValToLastHeightMap(state, stateDB, blockStore)
	if err != nil {
		return nil, err
	}

	return &Pool{
		stateDB:         stateDB,
		blockStore:      blockStore,
		state:           state,
		logger:          log.NewNopLogger(),
		store:           store,
		evidenceList:    clist.New(),
		valToLastHeight: valToLastHeight,
	}, nil
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

	evpool.updateValToLastHeight(block.Height, state)
}

// AddEvidence checks the evidence is valid and adds it to the pool. If
// evidence is composite (ConflictingHeadersEvidence), it will be broken up
// into smaller pieces.
func (evpool *Pool) AddEvidence(evidence types.Evidence) error {
	var (
		state  = evpool.State()
		evList = []types.Evidence{evidence}
	)

	valSet, err := sm.LoadValidators(evpool.stateDB, evidence.Height())
	if err != nil {
		return fmt.Errorf("can't load validators at height #%d: %w", evidence.Height(), err)
	}

	// Break composite evidence into smaller pieces.
	if ce, ok := evidence.(types.CompositeEvidence); ok {
		evpool.logger.Info("Breaking up composite evidence", "ev", evidence)

		blockMeta := evpool.blockStore.LoadBlockMeta(evidence.Height())
		if blockMeta == nil {
			return fmt.Errorf("don't have block meta at height #%d", evidence.Height())
		}

		if err := ce.VerifyComposite(&blockMeta.Header, valSet); err != nil {
			return err
		}

		evList = ce.Split(&blockMeta.Header, valSet, evpool.valToLastHeight)
	}

	for _, ev := range evList {
		if evpool.store.Has(evidence) {
			return ErrEvidenceAlreadyStored{}
		}

		// For lunatic validator evidence, a header needs to be fetched.
		var header *types.Header
		if _, ok := ev.(*types.LunaticValidatorEvidence); ok {
			blockMeta := evpool.blockStore.LoadBlockMeta(ev.Height())
			if blockMeta == nil {
				return fmt.Errorf("don't have block meta at height #%d", ev.Height())
			}
			header = &blockMeta.Header
		}

		// 1) Verify against state.
		if err := sm.VerifyEvidence(evpool.stateDB, state, ev, header); err != nil {
			return fmt.Errorf("failed to verify %v: %w", ev, err)
		}

		// 2) Compute priority.
		_, val := valSet.GetByAddress(ev.Address())
		priority := val.VotingPower

		// 3) Save to store.
		_, err := evpool.store.AddNewEvidence(ev, priority)
		if err != nil {
			return fmt.Errorf("failed to add new evidence %v: %w", ev, err)
		}

		// 4) Add evidence to clist.
		evpool.evidenceList.PushBack(ev)

		evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", ev)
	}

	return nil
}

// MarkEvidenceAsCommitted marks all the evidence as committed and removes it
// from the queue.
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

// IsCommitted returns true if we have already seen this exact evidence and it
// is already marked as committed.
func (evpool *Pool) IsCommitted(evidence types.Evidence) bool {
	ei := evpool.store.getInfo(evidence)
	return ei.Evidence != nil && ei.Committed
}

// ValidatorLastHeight returns the last height of the validator w/ the
// given address. 0 - if address never was a validator or was such a
// long time ago (> ConsensusParams.Evidence.MaxAgeDuration && >
// ConsensusParams.Evidence.MaxAgeNumBlocks).
func (evpool *Pool) ValidatorLastHeight(address []byte) int64 {
	h, ok := evpool.valToLastHeight[string(address)]
	if !ok {
		return 0
	}
	return h
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

func (evpool *Pool) updateValToLastHeight(blockHeight int64, state sm.State) {
	// Update current validators & add new ones.
	for _, val := range state.Validators.Validators {
		evpool.valToLastHeight[string(val.Address)] = blockHeight
	}

	// Remove validators outside of MaxAgeNumBlocks & MaxAgeDuration.
	removeHeight := blockHeight - evpool.State().ConsensusParams.Evidence.MaxAgeNumBlocks
	if removeHeight >= 1 {
		valSet, err := sm.LoadValidators(evpool.stateDB, removeHeight)
		if err != nil {
			for _, val := range valSet.Validators {
				h, ok := evpool.valToLastHeight[string(val.Address)]
				if ok && h == removeHeight {
					delete(evpool.valToLastHeight, string(val.Address))
				}
			}
		}
	}
}

func buildValToLastHeightMap(state sm.State, stateDB dbm.DB, blockStore *store.BlockStore) (valToLastHeightMap, error) {
	var (
		valToLastHeight = make(map[string]int64)
		params          = state.ConsensusParams.Evidence

		numBlocks  = int64(0)
		minAgeTime = time.Now().Add(-params.MaxAgeDuration)
		height     = state.LastBlockHeight
	)

	if height == 0 {
		return valToLastHeight, nil
	}

	meta := blockStore.LoadBlockMeta(height)
	if meta == nil {
		return nil, fmt.Errorf("block meta for height %d not found", height)
	}
	blockTime := meta.Header.Time

	// From state.LastBlockHeight, build a map of "active" validators until
	// MaxAgeNumBlocks is passed and block time is less than now() -
	// MaxAgeDuration.
	for height >= 1 && (numBlocks <= params.MaxAgeNumBlocks || !blockTime.Before(minAgeTime)) {
		valSet, err := sm.LoadValidators(stateDB, height)
		if err != nil {
			// last stored height -> return
			if _, ok := err.(sm.ErrNoValSetForHeight); ok {
				return valToLastHeight, nil
			}
			return nil, fmt.Errorf("validator set for height %d not found", height)
		}

		for _, val := range valSet.Validators {
			key := string(val.Address)
			if _, ok := valToLastHeight[key]; !ok {
				valToLastHeight[key] = height
			}
		}

		height--

		if height > 0 {
			// NOTE: we assume here blockStore and state.Validators are in sync. I.e if
			// block N is stored, then validators for height N are also stored in
			// state.
			meta := blockStore.LoadBlockMeta(height)
			if meta == nil {
				return nil, fmt.Errorf("block meta for height %d not found", height)
			}
			blockTime = meta.Header.Time
		}

		numBlocks++
	}

	return valToLastHeight, nil
}
