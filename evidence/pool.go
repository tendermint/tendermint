package evidence

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	dbm "github.com/tendermint/tm-db"

	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	baseKeyCommitted = byte(0x00)
	baseKeyPending   = byte(0x01)
)

// Pool maintains a pool of valid evidence to be broadcasted and committed
type Pool struct {
	logger log.Logger

	evidenceStore dbm.DB
	evidenceList  *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	stateDB sm.Store
	// needed to load headers to verify evidence
	blockStore BlockStore

	mtx sync.Mutex
	// latest state
	state sm.State
}

// NewPool creates an evidence pool. If using an existing evidence store,
// it will add all pending evidence to the concurrent list.
func NewPool(evidenceDB dbm.DB, stateDB sm.Store, blockStore BlockStore) (*Pool, error) {

	state, err := stateDB.Load()
	if err != nil {
		return nil, fmt.Errorf("cannot load state: %w", err)
	}

	pool := &Pool{
		stateDB:       stateDB,
		blockStore:    blockStore,
		state:         state,
		logger:        log.NewNopLogger(),
		evidenceStore: evidenceDB,
		evidenceList:  clist.New(),
	}

	// if pending evidence already in db, in event of prior failure, then load it back to the evidenceList
	evList := pool.AllPendingEvidence()
	for _, ev := range evList {
		pool.evidenceList.PushBack(ev)
	}

	return pool, nil
}

// PendingEvidence is used primarily as part of block proposal and returns up to maxNum of uncommitted evidence.
// If maxNum is -1, all evidence is returned. Pending evidence is prioritized based on time.
func (evpool *Pool) PendingEvidence(maxNum uint32) []types.Evidence {
	evpool.removeExpiredPendingEvidence()
	evidence, err := evpool.listEvidence(baseKeyPending, int64(maxNum))
	if err != nil {
		evpool.logger.Error("Unable to retrieve pending evidence", "err", err)
	}
	return evidence
}

// AllPendingEvidence returns all evidence ready to be proposed and committed.
func (evpool *Pool) AllPendingEvidence() []types.Evidence {
	evpool.removeExpiredPendingEvidence()
	evidence, err := evpool.listEvidence(baseKeyPending, -1)
	if err != nil {
		evpool.logger.Error("Unable to retrieve pending evidence", "err", err)
	}
	return evidence
}

// Update uses the latest block & state to update any evidence that has been committed, to prune all expired evidence
func (evpool *Pool) Update(block *types.Block, state sm.State) {
	// sanity check
	if state.LastBlockHeight != block.Height {
		panic(fmt.Sprintf("Failed EvidencePool.Update sanity check: got state.Height=%d with block.Height=%d",
			state.LastBlockHeight,
			block.Height,
		),
		)
	}

	// update the state
	evpool.updateState(state)

	// remove evidence from pending and mark committed
	evpool.MarkEvidenceAsCommitted(block.Height, block.Evidence.Evidence)

	// prune pending, committed and potential evidence and polc's periodically
	if block.Height%state.ConsensusParams.Evidence.MaxAgeNumBlocks == 0 {
		evpool.logger.Debug("Pruning expired evidence")
		// NOTE: As this is periodic, this implies that there may be some pending evidence in the
		// db that have already expired. However, expired evidence will also be removed whenever
		// PendingEvidence() is called ensuring that no expired evidence is proposed.
		evpool.removeExpiredPendingEvidence()
	}
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *Pool) AddEvidence(ev types.Evidence) error {
	evpool.logger.Debug("Attempting to add evidence", "ev", ev)

	if evpool.Has(ev) {
		return nil
	}

	// 1) Verify against state.
	if err := evpool.verify(ev); err != nil {
		return types.NewErrEvidenceInvalid(ev, err)
	}

	// 2) Save to store.
	if err := evpool.addPendingEvidence(ev); err != nil {
		return fmt.Errorf("database error when adding evidence: %v", err)
	}

	// 3) Add evidence to clist.
	evpool.evidenceList.PushBack(ev)

	evpool.logger.Info("Verified new evidence of byzantine behavior", "evidence", ev)

	return nil
}

// Verify verifies the evidence against the node's (or evidence pool's) state. More specifically, to validate
// evidence against state is to validate it against the nodes own header and validator set for that height. This ensures
// as well as meeting the evidence's own validation rules, that the evidence hasn't expired, that the validator is still
// bonded and that the evidence can be committed to the chain.
func (evpool *Pool) Verify(evidence types.Evidence) error {
	if evpool.IsCommitted(evidence) {
		return errors.New("evidence was already committed")
	}
	// We have already verified this piece of evidence - no need to do it again
	if evpool.IsPending(evidence) {
		return nil
	}

	return evpool.verify(evidence)
}

func (evpool *Pool) verify(evidence types.Evidence) error {
	return VerifyEvidence(evidence, evpool.State(), evpool.stateDB, evpool.blockStore)
}

// MarkEvidenceAsCommitted marks all the evidence as committed and removes it
// from the queue.
func (evpool *Pool) MarkEvidenceAsCommitted(height int64, evidence []types.Evidence) {
	// make a map of committed evidence to remove from the clist
	blockEvidenceMap := make(map[string]struct{})
	for _, ev := range evidence {
		// As the evidence is stored in the block store we only need to record the height that it was saved at.
		key := keyCommitted(ev)

		h := gogotypes.Int64Value{Value: height}
		evBytes, err := proto.Marshal(&h)
		if err != nil {
			panic(err)
		}

		if err := evpool.evidenceStore.Set(key, evBytes); err != nil {
			evpool.logger.Error("Unable to add committed evidence", "err", err)
			// if we can't move evidence to committed then don't remove the evidence from pending
			continue
		}
		// if pending, remove from that bucket, remember not all evidence has been seen before
		if evpool.IsPending(ev) {
			evpool.removePendingEvidence(ev)
			blockEvidenceMap[evMapKey(ev)] = struct{}{}
		}
	}

	// remove committed evidence from the clist
	if len(blockEvidenceMap) != 0 {
		evpool.removeEvidenceFromList(blockEvidenceMap)
	}
}

// Has checks whether the evidence exists either pending or already committed
func (evpool *Pool) Has(evidence types.Evidence) bool {
	return evpool.IsPending(evidence) || evpool.IsCommitted(evidence)
}

// IsEvidenceExpired checks whether evidence is past the maximum age where it can be used
func (evpool *Pool) IsEvidenceExpired(evidence types.Evidence) bool {
	return evpool.IsExpired(evidence.Height(), evidence.Time())
}

// IsExpired checks whether evidence or a polc is expired by checking whether a height and time is older
// than set by the evidence consensus parameters
func (evpool *Pool) IsExpired(height int64, time time.Time) bool {
	var (
		params       = evpool.State().ConsensusParams.Evidence
		ageDuration  = evpool.State().LastBlockTime.Sub(time)
		ageNumBlocks = evpool.State().LastBlockHeight - height
	)
	return ageNumBlocks > params.MaxAgeNumBlocks &&
		ageDuration > params.MaxAgeDuration
}

// IsCommitted returns true if we have already seen this exact evidence and it is already marked as committed.
func (evpool *Pool) IsCommitted(evidence types.Evidence) bool {
	key := keyCommitted(evidence)
	ok, err := evpool.evidenceStore.Has(key)
	if err != nil {
		evpool.logger.Error("Unable to find committed evidence", "err", err)
	}
	return ok
}

// IsPending checks whether the evidence is already pending. DB errors are passed to the logger.
func (evpool *Pool) IsPending(evidence types.Evidence) bool {
	key := keyPending(evidence)
	ok, err := evpool.evidenceStore.Has(key)
	if err != nil {
		evpool.logger.Error("Unable to find pending evidence", "err", err)
	}
	return ok
}

// EvidenceFront goes to the first evidence in the clist
func (evpool *Pool) EvidenceFront() *clist.CElement {
	return evpool.evidenceList.Front()
}

// EvidenceWaitChan is a channel that closes once the first evidence in the list is there. i.e Front is not nil
func (evpool *Pool) EvidenceWaitChan() <-chan struct{} {
	return evpool.evidenceList.WaitChan()
}

// SetLogger sets the Logger.
func (evpool *Pool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// State returns the current state of the evpool.
func (evpool *Pool) State() sm.State {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return evpool.state
}

func (evpool *Pool) addPendingEvidence(evidence types.Evidence) error {
	evi, err := types.EvidenceToProto(evidence)
	if err != nil {
		return fmt.Errorf("unable to convert to proto, err: %w", err)
	}

	evBytes, err := proto.Marshal(evi)
	if err != nil {
		return fmt.Errorf("unable to marshal evidence: %w", err)
	}

	key := keyPending(evidence)

	return evpool.evidenceStore.Set(key, evBytes)
}

func (evpool *Pool) removePendingEvidence(evidence types.Evidence) {
	key := keyPending(evidence)
	if err := evpool.evidenceStore.Delete(key); err != nil {
		evpool.logger.Error("Unable to delete pending evidence", "err", err)
	} else {
		evpool.logger.Info("Deleted pending evidence", "evidence", evidence)
	}
}

// listEvidence lists up to maxNum pieces of evidence for the given prefix key.
// If maxNum is -1, there's no cap on the size of returned evidence.
func (evpool *Pool) listEvidence(prefixKey byte, maxNum int64) ([]types.Evidence, error) {
	var count int64
	var evidence []types.Evidence
	iter, err := dbm.IteratePrefix(evpool.evidenceStore, []byte{prefixKey})
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		if count == maxNum {
			return evidence, nil
		}
		count++

		val := iter.Value()
		var (
			ev   types.Evidence
			evpb tmproto.Evidence
		)
		err := proto.Unmarshal(val, &evpb)
		if err != nil {
			return nil, err
		}

		ev, err = types.EvidenceFromProto(&evpb)
		if err != nil {
			return nil, err
		}

		evidence = append(evidence, ev)
	}

	return evidence, nil
}

func (evpool *Pool) removeExpiredPendingEvidence() {
	iter, err := dbm.IteratePrefix(evpool.evidenceStore, []byte{baseKeyPending})
	if err != nil {
		evpool.logger.Error("Unable to iterate over pending evidence", "err", err)
		return
	}
	defer iter.Close()
	blockEvidenceMap := make(map[string]struct{})
	for ; iter.Valid(); iter.Next() {
		evBytes := iter.Value()
		var (
			ev   types.Evidence
			evpb tmproto.Evidence
		)
		err := proto.Unmarshal(evBytes, &evpb)
		if err != nil {
			evpool.logger.Error("Unable to unmarshal Evidence", "err", err)
			continue
		}

		ev, err = types.EvidenceFromProto(&evpb)
		if err != nil {
			evpool.logger.Error("Error in transition evidence from protobuf", "err", err)
			continue
		}
		if !evpool.IsExpired(ev.Height()-1, ev.Time()) {
			if len(blockEvidenceMap) != 0 {
				evpool.removeEvidenceFromList(blockEvidenceMap)
			}

			return
		}
		evpool.removePendingEvidence(ev)
		blockEvidenceMap[evMapKey(ev)] = struct{}{}
	}
}

func (evpool *Pool) removeEvidenceFromList(
	blockEvidenceMap map[string]struct{}) {

	for e := evpool.evidenceList.Front(); e != nil; e = e.Next() {
		// Remove from clist
		ev := e.Value.(types.Evidence)
		if _, ok := blockEvidenceMap[evMapKey(ev)]; ok {
			evpool.evidenceList.Remove(e)
			e.DetachPrev()
		}
	}
}

func (evpool *Pool) updateState(state sm.State) {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	evpool.state = state
}

func evMapKey(ev types.Evidence) string {
	return string(ev.Hash())
}

// big endian padded hex
func bE(h int64) string {
	return fmt.Sprintf("%0.16X", h)
}

func keyCommitted(evidence types.Evidence) []byte {
	return append([]byte{baseKeyCommitted}, keySuffix(evidence)...)
}

func keyPending(evidence types.Evidence) []byte {
	return append([]byte{baseKeyPending}, keySuffix(evidence)...)
}

func keySuffix(evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("%s/%X", bE(evidence.Height()), evidence.Hash()))
}
