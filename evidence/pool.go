package evidence

import (
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	dbm "github.com/tendermint/tm-db"

	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

const (
	baseKeyCommitted = byte(0x00)
	baseKeyPending   = byte(0x01)
	baseKeyPOLC      = byte(0x02)
)

// Pool maintains a pool of valid evidence to be broadcasted and committed
type Pool struct {
	logger log.Logger

	evidenceStore dbm.DB
	evidenceList  *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	stateDB dbm.DB
	// needed to load headers to verify evidence
	blockStore *store.BlockStore

	mtx sync.Mutex
	// latest state
	state sm.State
	// a map of active validators and respective last heights validator is active
	// if it was in validator set after EvidenceParams.MaxAgeNumBlocks or
	// currently is (ie. [MaxAgeNumBlocks, CurrentHeight])
	// In simple words, it means it's still bonded -> therefore slashable.
	valToLastHeight valToLastHeightMap
}

// Validator.Address -> Last height it was in validator set
type valToLastHeightMap map[string]int64

func NewPool(stateDB, evidenceDB dbm.DB, blockStore *store.BlockStore) (*Pool, error) {
	var (
		state = sm.LoadState(stateDB)
	)

	valToLastHeight, err := buildValToLastHeightMap(state, stateDB, blockStore)
	if err != nil {
		return nil, err
	}

	pool := &Pool{
		stateDB:         stateDB,
		blockStore:      blockStore,
		state:           state,
		logger:          log.NewNopLogger(),
		evidenceStore:   evidenceDB,
		evidenceList:    clist.New(),
		valToLastHeight: valToLastHeight,
	}

	// if pending evidence already in db, in event of prior failure, then load it back to the evidenceList
	evList := pool.AllPendingEvidence()
	for _, ev := range evList {
		if pool.IsEvidenceExpired(ev) {
			pool.removePendingEvidence(ev)
			continue
		}
		pool.evidenceList.PushBack(ev)
	}

	return pool, nil
}

// PendingEvidence is used primarily as part of block proposal and returns up to maxNum of uncommitted evidence.
// If maxNum is -1, all evidence is returned. Pending evidence is prioritised based on time.
func (evpool *Pool) PendingEvidence(maxNum uint32) []types.Evidence {
	evidence, err := evpool.listEvidence(baseKeyPending, int64(maxNum))
	if err != nil {
		evpool.logger.Error("Unable to retrieve pending evidence", "err", err)
	}
	return evidence
}

func (evpool *Pool) AllPendingEvidence() []types.Evidence {
	evidence, err := evpool.listEvidence(baseKeyPending, -1)
	if err != nil {
		evpool.logger.Error("Unable to retrieve pending evidence", "err", err)
	}
	return evidence
}

// Update uses the latest block & state to update its copy of the state,
// validator to last height map and calls MarkEvidenceAsCommitted.
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

	// remove evidence from pending and mark committed
	evpool.MarkEvidenceAsCommitted(block.Height, block.Time, block.Evidence.Evidence)

	// remove expired evidence - this should be done at every height to ensure we don't send expired evidence to peers
	evpool.removeExpiredPendingEvidence()

	// as it's not vital to remove expired POLCs, we only prune periodically
	if block.Height%state.ConsensusParams.Evidence.MaxAgeNumBlocks == 0 {
		evpool.pruneExpiredPOLC()
	}

	// update the state
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	evpool.state = state
	evpool.updateValToLastHeight(block.Height, state)
}

// AddPOLC adds a proof of lock change to the evidence database
// that may be needed in the future to verify votes
func (evpool *Pool) AddPOLC(polc types.ProofOfLockChange) error {
	key := keyPOLC(polc)
	pbplc, err := polc.ToProto()
	if err != nil {
		return err
	}
	polcBytes, err := proto.Marshal(pbplc)
	if err != nil {
		return fmt.Errorf("addPOLC: unable to marshal ProofOfLockChange: %w", err)
	}
	return evpool.evidenceStore.Set(key, polcBytes)
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

		// XXX: Copy here since this should be a rare case.
		evpool.mtx.Lock()
		valToLastHeightCopy := make(valToLastHeightMap, len(evpool.valToLastHeight))
		for k, v := range evpool.valToLastHeight {
			valToLastHeightCopy[k] = v
		}
		evpool.mtx.Unlock()

		evList = ce.Split(&blockMeta.Header, valSet, valToLastHeightCopy)
	}

	for _, ev := range evList {
		if evpool.Has(ev) {
			continue
		}

		// For lunatic validator evidence, a header needs to be fetched.
		var header *types.Header
		if _, ok := ev.(*types.LunaticValidatorEvidence); ok {
			header = evpool.Header(ev.Height())
			if header == nil {
				return fmt.Errorf("don't have block meta at height #%d", ev.Height())
			}
		}

		// 1) Verify against state.
		if err := sm.VerifyEvidence(evpool.stateDB, state, ev, header); err != nil {
			return fmt.Errorf("failed to verify %v: %w", ev, err)
		}

		// 2) Save to store.
		if err := evpool.addPendingEvidence(ev); err != nil {
			return fmt.Errorf("database error: %v", err)
		}

		// 3) Add evidence to clist.
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

// RetrievePOLC attempts to find a polc at the given height and round, if not there it returns an error
func (evpool *Pool) RetrievePOLC(height int64, round int32) (polc types.ProofOfLockChange, err error) {
	var pbpolc tmproto.ProofOfLockChange
	key := keyPOLCFromHeightAndRound(height, round)
	polcBytes, err := evpool.evidenceStore.Get(key)
	if err != nil {
		return polc, err
	}

	if polcBytes == nil {
		return polc, fmt.Errorf("unable to find polc at height %d and round %d", height, round)
	}

	err = proto.Unmarshal(polcBytes, &pbpolc)
	if err != nil {
		return polc, err
	}
	plc, err := types.ProofOfLockChangeFromProto(&pbpolc)
	if err != nil {
		return polc, err
	}

	return *plc, err
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

// Header gets the header from the block store at a specified height.
// Is used for validation of LunaticValidatorEvidence
func (evpool *Pool) Header(height int64) *types.Header {
	blockMeta := evpool.blockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}
	return &blockMeta.Header
}

// ValidatorLastHeight returns the last height of the validator w/ the
// given address. 0 - if address never was a validator or was such a
// long time ago (> ConsensusParams.Evidence.MaxAgeDuration && >
// ConsensusParams.Evidence.MaxAgeNumBlocks).
func (evpool *Pool) ValidatorLastHeight(address []byte) int64 {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()

	h, ok := evpool.valToLastHeight[string(address)]
	if !ok {
		return 0
	}
	return h
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
		return err
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
		val := iter.Value()

		if count == maxNum {
			return evidence, nil
		}
		count++

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

func (evpool *Pool) pruneExpiredPOLC() {
	evpool.logger.Debug("Pruning expired POLC's")
	iter, err := dbm.IteratePrefix(evpool.evidenceStore, []byte{baseKeyPOLC})
	if err != nil {
		evpool.logger.Error("Unable to iterate over POLC's", "err", err)
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		proofBytes := iter.Value()
		var (
			pbproof tmproto.ProofOfLockChange
		)
		err := proto.Unmarshal(proofBytes, &pbproof)
		if err != nil {
			evpool.logger.Error("Unable to unmarshal POLC", "err", err)
			continue
		}
		proof, err := types.ProofOfLockChangeFromProto(&pbproof)
		if err != nil {
			evpool.logger.Error("Unable to transition POLC from protobuf", "err", err)
			continue
		}
		if !evpool.IsExpired(proof.Height()-1, proof.Time()) {
			return
		}
		err = evpool.evidenceStore.Delete(iter.Key())
		if err != nil {
			evpool.logger.Error("Unable to delete expired POLC", "err", err)
			continue
		}
		evpool.logger.Info("Deleted expired POLC", "polc", proof)
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
	removeHeight := blockHeight - state.ConsensusParams.Evidence.MaxAgeNumBlocks
	if removeHeight >= 1 {
		for val, height := range evpool.valToLastHeight {
			if height <= removeHeight {
				delete(evpool.valToLastHeight, val)
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

func keyPOLC(polc types.ProofOfLockChange) []byte {
	return keyPOLCFromHeightAndRound(polc.Height(), polc.Round())
}

func keyPOLCFromHeightAndRound(height int64, round int32) []byte {
	return append([]byte{baseKeyPOLC}, []byte(fmt.Sprintf("%s/%s", bE(height), bE(int64(round))))...)
}

func keySuffix(evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("%s/%X", bE(evidence.Height()), evidence.Hash()))
}

// ErrInvalidEvidence returns when evidence failed to validate
type ErrInvalidEvidence struct {
	Reason error
}

func (e ErrInvalidEvidence) Error() string {
	return fmt.Sprintf("evidence is not valid: %v ", e.Reason)
}
