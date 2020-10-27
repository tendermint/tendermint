package evidence

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	evproto "github.com/tendermint/tendermint/proto/tendermint/evidence"
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
	evidenceSize  uint32       // amount of pending evidence

	// needed to load validators to verify evidence
	stateDB sm.Store
	// needed to load headers and commits to verify evidence
	blockStore BlockStore

	mtx sync.Mutex
	// latest state
	state sm.State

	pruningHeight int64
	pruningTime   time.Time
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

	// if pending evidence already in db, in event of prior failure, then check for expiration,
	// update the size and load it back to the evidenceList
	pool.pruningHeight, pool.pruningTime = pool.removeExpiredPendingEvidence()
	evList, _, err := pool.listEvidence(baseKeyPending, -1)
	if err != nil {
		return nil, err
	}
	atomic.StoreUint32(&pool.evidenceSize, uint32(len(evList)))
	for _, ev := range evList {
		pool.evidenceList.PushBack(ev)
	}

	return pool, nil
}

// PendingEvidence is used primarily as part of block proposal and returns up to maxNum of uncommitted evidence.
func (evpool *Pool) PendingEvidence(maxBytes int64) ([]types.Evidence, int64) {
	if evpool.Size() == 0 {
		return []types.Evidence{}, 0
	}
	evidence, size, err := evpool.listEvidence(baseKeyPending, maxBytes)
	if err != nil {
		evpool.logger.Error("Unable to retrieve pending evidence", "err", err)
	}
	return evidence, size
}

// Update pulls the latest state to be used for expiration and evidence params and then prunes all expired evidence
func (evpool *Pool) Update(state sm.State) {
	// sanity check
	if state.LastBlockHeight <= evpool.state.LastBlockHeight {
		panic(fmt.Sprintf(
			"Failed EvidencePool.Update new state height is less than or equal to previous state height: %d <= %d",
			state.LastBlockHeight,
			evpool.state.LastBlockHeight,
		))
	}
	evpool.logger.Info("Updating evidence pool", "last_block_height", state.LastBlockHeight,
		"last_block_time", state.LastBlockTime)

	// update the state
	evpool.updateState(state)

	// prune pending evidence when it has expired. This also updates when the next evidence will expire
	if evpool.Size() > 0 && state.LastBlockHeight > evpool.pruningHeight &&
		state.LastBlockTime.After(evpool.pruningTime) {
		evpool.pruningHeight, evpool.pruningTime = evpool.removeExpiredPendingEvidence()
	}
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *Pool) AddEvidence(ev types.Evidence) error {
	evpool.logger.Debug("Attempting to add evidence", "ev", ev)

	// We have already verified this piece of evidence - no need to do it again
	if evpool.isPending(ev) {
		evpool.logger.Info("Evidence already pending, ignoring this one", "ev", ev)
		return nil
	}

	// check that the evidence isn't already committed
	if evpool.isCommitted(ev) {
		// this can happen if the peer that sent us the evidence is behind so we shouldn't
		// punish the peer.
		evpool.logger.Debug("Evidence was already committed, ignoring this one", "ev", ev)
		return nil
	}

	// 1) Verify against state.
	evInfo, err := evpool.verify(ev)
	if err != nil {
		return types.NewErrInvalidEvidence(ev, err)
	}

	// 2) Save to store.
	if err := evpool.addPendingEvidence(evInfo); err != nil {
		return fmt.Errorf("can't add evidence to pending list: %w", err)
	}

	// 3) Add evidence to clist.
	evpool.evidenceList.PushBack(ev)

	evpool.logger.Info("Verified new evidence of byzantine behavior", "evidence", ev)

	return nil
}

// AddEvidenceFromConsensus should be exposed only to the consensus so it can add evidence to the pool
// directly without the need for verification.
func (evpool *Pool) AddEvidenceFromConsensus(ev types.Evidence, time time.Time, valSet *types.ValidatorSet) error {
	var (
		vals       []*types.Validator
		totalPower int64
	)

	// we already have this evidence, log this but don't return an error.
	if evpool.isPending(ev) {
		evpool.logger.Info("Evidence already pending, ignoring this one", "ev", ev)
		return nil
	}

	switch ev := ev.(type) {
	case *types.DuplicateVoteEvidence:
		_, val := valSet.GetByAddress(ev.VoteA.ValidatorAddress)
		vals = append(vals, val)
		totalPower = valSet.TotalVotingPower()
	default:
		return fmt.Errorf("unrecognized evidence type: %T", ev)
	}

	evInfo := &info{
		Evidence:         ev,
		Time:             time,
		Validators:       vals,
		TotalVotingPower: totalPower,
	}

	if err := evpool.addPendingEvidence(evInfo); err != nil {
		return fmt.Errorf("can't add evidence to pending list: %w", err)
	}
	// add evidence to be gossiped with peers
	evpool.evidenceList.PushBack(ev)

	evpool.logger.Info("Verified new evidence of byzantine behavior", "evidence", ev)

	return nil
}

// CheckEvidence takes an array of evidence from a block and verifies all the evidence there.
// If it has already verified the evidence then it jumps to the next one. It ensures that no
// evidence has already been committed or is being proposed twice. It also adds any
// evidence that it doesn't currently have so that it can quickly form ABCI Evidence later.
func (evpool *Pool) CheckEvidence(evList types.EvidenceList) error {
	hashes := make([][]byte, len(evList))
	for idx, ev := range evList {

		ok := evpool.fastCheck(ev)

		if !ok {
			// check that the evidence isn't already committed
			if evpool.isCommitted(ev) {
				return &types.ErrInvalidEvidence{Evidence: ev, Reason: errors.New("evidence was already committed")}
			}

			evInfo, err := evpool.verify(ev)
			if err != nil {
				return &types.ErrInvalidEvidence{Evidence: ev, Reason: err}
			}

			if err := evpool.addPendingEvidence(evInfo); err != nil {
				// Something went wrong with adding the evidence but we already know it is valid
				// hence we log an error and continue
				evpool.logger.Error("Can't add evidence to pending list", "err", err, "evInfo", evInfo)
			}

			evpool.logger.Info("Verified new evidence of byzantine behavior", "evidence", ev)
		}

		// check for duplicate evidence. We cache hashes so we don't have to work them out again.
		hashes[idx] = ev.Hash()
		for i := idx - 1; i >= 0; i-- {
			if bytes.Equal(hashes[i], hashes[idx]) {
				return &types.ErrInvalidEvidence{Evidence: ev, Reason: errors.New("duplicate evidence")}
			}
		}
	}

	return nil
}

// ABCIEvidence processes all the evidence in the block, marking it as committed and removing it
// from the pending database. It then forms the individual abci evidence that will be passed back to
// the application.
func (evpool *Pool) ABCIEvidence(height int64, evidence []types.Evidence) []abci.Evidence {
	// make a map of committed evidence to remove from the clist
	blockEvidenceMap := make(map[string]struct{}, len(evidence))
	abciEvidence := make([]abci.Evidence, 0)
	for _, ev := range evidence {

		// get entire evidence info from pending list
		infoBytes, err := evpool.evidenceStore.Get(keyPending(ev))
		if err != nil {
			evpool.logger.Error("Unable to retrieve evidence to pass to ABCI. "+
				"Evidence pool should have seen this evidence before",
				"evidence", ev, "err", err)
			continue
		}
		var infoProto evproto.Info
		err = infoProto.Unmarshal(infoBytes)
		if err != nil {
			evpool.logger.Error("Decoding evidence info failed", "err", err, "height", ev.Height(), "hash", ev.Hash())
			continue
		}
		evInfo, err := infoFromProto(&infoProto)
		if err != nil {
			evpool.logger.Error("Converting evidence info from proto failed", "err", err, "height", ev.Height(),
				"hash", ev.Hash())
			continue
		}

		var evType abci.EvidenceType
		switch ev.(type) {
		case *types.DuplicateVoteEvidence:
			evType = abci.EvidenceType_DUPLICATE_VOTE
		case *types.LightClientAttackEvidence:
			evType = abci.EvidenceType_LIGHT_CLIENT_ATTACK
		default:
			evpool.logger.Error("Unknown evidence type", "T", reflect.TypeOf(ev))
			continue
		}
		for _, val := range evInfo.Validators {
			abciEv := abci.Evidence{
				Type:             evType,
				Validator:        types.TM2PB.Validator(val),
				Height:           ev.Height(),
				Time:             evInfo.Time,
				TotalVotingPower: evInfo.TotalVotingPower,
			}
			abciEvidence = append(abciEvidence, abciEv)
			evpool.logger.Info("Created ABCI evidence", "ev", abciEv)
		}

		// we can now remove the evidence from the pending list and the clist that we use for gossiping
		evpool.removePendingEvidence(ev)
		blockEvidenceMap[evMapKey(ev)] = struct{}{}

		// Add evidence to the committed list
		// As the evidence is stored in the block store we only need to record the height that it was saved at.
		key := keyCommitted(ev)

		h := gogotypes.Int64Value{Value: height}
		evBytes, err := proto.Marshal(&h)
		if err != nil {
			panic(err)
		}

		if err := evpool.evidenceStore.Set(key, evBytes); err != nil {
			evpool.logger.Error("Unable to add committed evidence", "err", err)
		}
	}

	// remove committed evidence from the clist
	if len(blockEvidenceMap) != 0 {
		evpool.removeEvidenceFromList(blockEvidenceMap)
	}

	return abciEvidence
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

func (evpool *Pool) Size() uint32 {
	return atomic.LoadUint32(&evpool.evidenceSize)
}

// State returns the current state of the evpool.
func (evpool *Pool) State() sm.State {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return evpool.state
}

//--------------------------------------------------------------------------

// Info is a wrapper around the evidence that the evidence pool receives with extensive
// information of what validators were malicious, the time of the attack and the total voting power
// This is saved as a form of cache so that the evidence pool can easily produce the ABCI Evidence
// needed to be sent to the application.
type info struct {
	Evidence         types.Evidence
	Time             time.Time
	Validators       []*types.Validator
	TotalVotingPower int64
	ByteSize         int64
}

// ToProto encodes into protobuf
func (ei info) ToProto() (*evproto.Info, error) {
	evpb, err := types.EvidenceToProto(ei.Evidence)
	if err != nil {
		return nil, err
	}

	valsProto := make([]*tmproto.Validator, len(ei.Validators))
	for i := 0; i < len(ei.Validators); i++ {
		valp, err := ei.Validators[i].ToProto()
		if err != nil {
			return nil, err
		}
		valsProto[i] = valp
	}

	return &evproto.Info{
		Evidence:         *evpb,
		Time:             ei.Time,
		Validators:       valsProto,
		TotalVotingPower: ei.TotalVotingPower,
	}, nil
}

// InfoFromProto decodes from protobuf into Info
func infoFromProto(proto *evproto.Info) (info, error) {
	if proto == nil {
		return info{}, errors.New("nil evidence info")
	}

	ev, err := types.EvidenceFromProto(&proto.Evidence)
	if err != nil {
		return info{}, err
	}

	vals := make([]*types.Validator, len(proto.Validators))
	for i := 0; i < len(proto.Validators); i++ {
		val, err := types.ValidatorFromProto(proto.Validators[i])
		if err != nil {
			return info{}, err
		}
		vals[i] = val
	}

	return info{
		Evidence:         ev,
		Time:             proto.Time,
		Validators:       vals,
		TotalVotingPower: proto.TotalVotingPower,
		ByteSize:         int64(proto.Evidence.Size()),
	}, nil

}

//--------------------------------------------------------------------------

// fastCheck leverages the fact that the evidence pool may have already verified the evidence to see if it can
// quickly conclude that the evidence is already valid.
func (evpool *Pool) fastCheck(ev types.Evidence) bool {
	key := keyPending(ev)
	if lcae, ok := ev.(*types.LightClientAttackEvidence); ok {
		evBytes, err := evpool.evidenceStore.Get(key)
		if evBytes == nil { // the evidence is not in the nodes pending list
			return false
		}
		if err != nil {
			evpool.logger.Error("Failed to load evidence", "err", err, "evidence", lcae)
			return false
		}
		evInfo, err := bytesToInfo(evBytes)
		if err != nil {
			evpool.logger.Error("Failed to convert evidence from proto", "err", err, "evidence", lcae)
			return false
		}
		// ensure that all the validators that the evidence pool have found to be malicious
		// are present in the list of commit signatures in the conflicting block
	OUTER:
		for _, sig := range lcae.ConflictingBlock.Commit.Signatures {
			for _, val := range evInfo.Validators {
				if bytes.Equal(val.Address, sig.ValidatorAddress) {
					continue OUTER
				}
			}
			// a validator we know is malicious is not included in the commit
			evpool.logger.Info("Fast check failed: a validator we know is malicious is not " +
				"in the commit sigs. Reverting to full verification")
			return false
		}
		return true
	}

	// for all other evidence the evidence pool just checks if it is already in the pending db
	return evpool.isPending(ev)
}

// IsExpired checks whether evidence or a polc is expired by checking whether a height and time is older
// than set by the evidence consensus parameters
func (evpool *Pool) isExpired(height int64, time time.Time) bool {
	var (
		params       = evpool.State().ConsensusParams.Evidence
		ageDuration  = evpool.State().LastBlockTime.Sub(time)
		ageNumBlocks = evpool.State().LastBlockHeight - height
	)
	return ageNumBlocks > params.MaxAgeNumBlocks &&
		ageDuration > params.MaxAgeDuration
}

// IsCommitted returns true if we have already seen this exact evidence and it is already marked as committed.
func (evpool *Pool) isCommitted(evidence types.Evidence) bool {
	key := keyCommitted(evidence)
	ok, err := evpool.evidenceStore.Has(key)
	if err != nil {
		evpool.logger.Error("Unable to find committed evidence", "err", err)
	}
	return ok
}

// IsPending checks whether the evidence is already pending. DB errors are passed to the logger.
func (evpool *Pool) isPending(evidence types.Evidence) bool {
	key := keyPending(evidence)
	ok, err := evpool.evidenceStore.Has(key)
	if err != nil {
		evpool.logger.Error("Unable to find pending evidence", "err", err)
	}
	return ok
}

func (evpool *Pool) addPendingEvidence(evInfo *info) error {
	evpb, err := evInfo.ToProto()
	if err != nil {
		return fmt.Errorf("unable to convert to proto, err: %w", err)
	}

	evBytes, err := evpb.Marshal()
	if err != nil {
		return fmt.Errorf("unable to marshal evidence: %w", err)
	}

	key := keyPending(evInfo.Evidence)

	err = evpool.evidenceStore.Set(key, evBytes)
	if err != nil {
		return fmt.Errorf("can't persist evidence: %w", err)
	}
	atomic.AddUint32(&evpool.evidenceSize, 1)
	return nil
}

func (evpool *Pool) removePendingEvidence(evidence types.Evidence) {
	key := keyPending(evidence)
	if err := evpool.evidenceStore.Delete(key); err != nil {
		evpool.logger.Error("Unable to delete pending evidence", "err", err)
	} else {
		atomic.AddUint32(&evpool.evidenceSize, ^uint32(0))
		evpool.logger.Info("Deleted pending evidence", "evidence", evidence)
	}
}

// listEvidence retrieves lists evidence from oldest to newest within maxBytes.
// If maxBytes is -1, there's no cap on the size of returned evidence.
func (evpool *Pool) listEvidence(prefixKey byte, maxBytes int64) ([]types.Evidence, int64, error) {
	var totalSize int64
	var evidence []types.Evidence
	iter, err := dbm.IteratePrefix(evpool.evidenceStore, []byte{prefixKey})
	if err != nil {
		return nil, totalSize, fmt.Errorf("database error: %v", err)
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		evInfo, err := bytesToInfo(iter.Value())
		if err != nil {
			return nil, totalSize, err
		}

		totalSize += evInfo.ByteSize

		if maxBytes != -1 && totalSize > maxBytes {
			return evidence, totalSize - evInfo.ByteSize, nil
		}

		evidence = append(evidence, evInfo.Evidence)
	}

	return evidence, totalSize, nil
}

func (evpool *Pool) removeExpiredPendingEvidence() (int64, time.Time) {
	iter, err := dbm.IteratePrefix(evpool.evidenceStore, []byte{baseKeyPending})
	if err != nil {
		evpool.logger.Error("Unable to iterate over pending evidence", "err", err)
		return evpool.State().LastBlockHeight, evpool.State().LastBlockTime
	}
	defer iter.Close()
	blockEvidenceMap := make(map[string]struct{})
	for ; iter.Valid(); iter.Next() {
		evInfo, err := bytesToInfo(iter.Value())
		if err != nil {
			evpool.logger.Error("Error in transition evidence from protobuf", "err", err)
			continue
		}
		if !evpool.isExpired(evInfo.Evidence.Height(), evInfo.Time) {
			if len(blockEvidenceMap) != 0 {
				evpool.removeEvidenceFromList(blockEvidenceMap)
			}

			// return the height and time with which this evidence will have expired so we know when to prune next
			return evInfo.Evidence.Height() + evpool.State().ConsensusParams.Evidence.MaxAgeNumBlocks + 1,
				evInfo.Time.Add(evpool.State().ConsensusParams.Evidence.MaxAgeDuration).Add(time.Second)
		}
		evpool.removePendingEvidence(evInfo.Evidence)
		blockEvidenceMap[evMapKey(evInfo.Evidence)] = struct{}{}
	}
	// We either have no pending evidence or all evidence has expired
	if len(blockEvidenceMap) != 0 {
		evpool.removeEvidenceFromList(blockEvidenceMap)
	}
	return evpool.State().LastBlockHeight, evpool.State().LastBlockTime
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

func bytesToInfo(evBytes []byte) (info, error) {
	var evpb evproto.Info
	err := evpb.Unmarshal(evBytes)
	if err != nil {
		return info{}, err
	}

	return infoFromProto(&evpb)
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
