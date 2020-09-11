package evidence

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	dbm "github.com/tendermint/tm-db"

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
	evidenceSize  uint16       // amount of pending evidence

	// needed to load validators to verify evidence
	stateDB StateStore
	// needed to load headers to verify evidence
	blockStore BlockStore

	mtx sync.Mutex
	// latest state
	state sm.State

	pruningHeight int64
	pruningTime   time.Time
}

// NewPool creates an evidence pool. If using an existing evidence store,
// it will add all pending evidence to the concurrent list.
func NewPool(evidenceDB dbm.DB, stateDB StateStore, blockStore BlockStore) (*Pool, error) {
	var (
		state = stateDB.LoadState()
	)

	pool := &Pool{
		stateDB:       stateDB,
		blockStore:    blockStore,
		state:         state,
		logger:        log.NewNopLogger(),
		evidenceStore: evidenceDB,
		evidenceList:  clist.New(),
		pruningHeight: state.LastBlockHeight,
		pruningTime:   state.LastBlockTime,
	}

	// if pending evidence already in db, in event of prior failure, then load it back to the evidenceList
	evList := pool.allPendingEvidence()
	for _, ev := range evList {
		pool.evidenceList.PushBack(ev)
	}

	return pool, nil
}

// PendingEvidence is used primarily as part of block proposal and returns up to maxNum of uncommitted evidence.
func (evpool *Pool) PendingEvidence(maxNum uint32) []types.Evidence {
	evpool.removeExpiredPendingEvidence()
	evidence, err := evpool.listEvidence(baseKeyPending, int64(maxNum))
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
		))
	}
	evpool.logger.Info("Updating evidence pool", "height", state.LastBlockHeight, "time", state.LastBlockTime)

	// update the state
	evpool.updateState(state)

	// remove evidence from pending and mark committed
	evpool.markEvidenceAsCommitted(block.Height, block.Evidence.Evidence)

	// prune pending evidence when it has expired. This also updates when the next evidence will expire
	if evpool.evidenceSize > 0 && state.LastBlockHeight > evpool.pruningHeight &&
		state.LastBlockTime.After(evpool.pruningTime) {
		evpool.pruningHeight, evpool.pruningTime = evpool.removeExpiredPendingEvidence()
	}
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *Pool) AddEvidence(ev types.Evidence) error {
	evpool.logger.Debug("Attempting to add evidence", "ev", ev)

	// We have already verified this piece of evidence - no need to do it again
	if evpool.isPending(ev) {
		return fmt.Errorf("evidence already verified and added, %s", ev.String())
	}

	// 1) Verify against state.
	evInfo, err := evpool.verify(ev)
	if err != nil {
		return types.NewErrEvidenceInvalid(ev, err)
	}

	// 2) Save to store.
	if err := evpool.addPendingEvidence(evInfo); err != nil {
		return fmt.Errorf("database error when adding evidence: %w", err)
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

	if evpool.isPending(ev) {
		return fmt.Errorf("evidence already verified and added, %s", ev.String()) // we already have this evidence
	}

	switch ev := ev.(type) {
	case *types.DuplicateVoteEvidence:
		_, val := valSet.GetByAddress(ev.VoteA.ValidatorAddress)
		vals = append(vals, val)
		totalPower = valSet.TotalVotingPower()
	default:
		return fmt.Errorf("unrecognized evidence: %v", ev)
	}

	evInfo := &Info{
		Evidence:         ev,
		Time:             time,
		Validators:       vals,
		TotalVotingPower: totalPower,
	}

	if err := evpool.addPendingEvidence(evInfo); err != nil {
		return fmt.Errorf("database error when adding evidence from consensus: %w", err)
	}

	evpool.evidenceList.PushBack(ev)

	evpool.logger.Info("Verified new evidence of byzantine behavior", "evidence", ev)

	return nil
}

// CheckEvidence takes an array of evidence from a block and verifies all the evidence there.
// If it has already verified the evidence then it jumps to the next one. It ensures that no
// evidence has already been committed or is being proposed twice. It also adds any
// evidence that it doesn't currently have so that it can quickly form ABCI Evidence later.
// Check evidence returns the hash of the entire evidence list so that it can be compares with
// the headers EvidenceHash.
func (evpool *Pool) CheckEvidence(evList types.EvidenceList) error {
	hashes := make([][]byte, len(evList))
	for idx, ev := range evList {

		ok := evpool.fastCheck(ev)

		if !ok {
			evInfo, err := evpool.verify(ev)
			if err != nil {
				return &types.ErrEvidenceInvalid{Evidence: ev, Reason: err}
			}

			if err := evpool.addPendingEvidence(evInfo); err != nil {
				evpool.logger.Error("Database error when adding evidence: %w", err)
			}
		}

		// check for duplicate evidence. We cache hashes so we don't have to work them out again.
		hashes[idx] = ev.Hash()
		for i := idx - 1; i >= 0; i-- {
			if bytes.Equal(hashes[i], hashes[idx]) {
				return &types.ErrEvidenceInvalid{Evidence: ev, Reason: errors.New("duplicate evidence")}
			}
		}
	}

	return nil
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

//--------------------------------------------------------------------------

// Info is a wrapper around the evidence that the evidence pool receives with extensive
// information of what validators were malicious, the time of the attack and the total voting power
// This is saved as a form of cache so that the evidence pool can easily produce the ABCI Evidence
// needed to be sent to the application.
type Info struct {
	Evidence         types.Evidence
	Time             time.Time
	Validators       []*types.Validator
	TotalVotingPower int64
}

// ToProto encodes into protobuf
func (ei Info) ToProto() (*evproto.Info, error) {
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
func InfoFromProto(proto *evproto.Info) (Info, error) {
	if proto == nil {
		return Info{}, errors.New("nil evidence info")
	}

	ev, err := types.EvidenceFromProto(&proto.Evidence)
	if err != nil {
		return Info{}, err
	}

	vals := make([]*types.Validator, len(proto.Validators))
	for i := 0; i < len(proto.Validators); i++ {
		val, err := types.ValidatorFromProto(proto.Validators[i])
		if err != nil {
			return Info{}, err
		}
		vals[i] = val
	}

	return Info{
		Evidence:         ev,
		Time:             proto.Time,
		Validators:       vals,
		TotalVotingPower: proto.TotalVotingPower,
	}, nil

}

//--------------------------------------------------------------------------

// allPendingEvidence returns all evidence ready to be proposed and committed.
func (evpool *Pool) allPendingEvidence() []types.Evidence {
	evpool.removeExpiredPendingEvidence()
	evidence, err := evpool.listEvidence(baseKeyPending, -1)
	if err != nil {
		evpool.logger.Error("Unable to retrieve pending evidence", "err", err)
	}
	// update pool size
	evpool.evidenceSize = uint16(len(evidence))
	return evidence
}

// markEvidenceAsCommitted marks all the evidence as committed and removes it
// from the queue.
func (evpool *Pool) markEvidenceAsCommitted(height int64, evidence []types.Evidence) {
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
		if evpool.isPending(ev) {
			evpool.removePendingEvidence(ev)
			blockEvidenceMap[evMapKey(ev)] = struct{}{}
		}
	}

	// remove committed evidence from the clist
	if len(blockEvidenceMap) != 0 {
		evpool.removeEvidenceFromList(blockEvidenceMap)
	}
}

// fastCheck leverages the fact that the evidence pool may have already verified the evidence to see if it can
// quickly conclude that the evidence is already valid.
func (evpool *Pool) fastCheck(ev types.Evidence) bool {
	key := keyPending(ev)
	if lcae, ok := ev.(*types.LightClientAttackEvidence); ok {
		evBytes, err := evpool.evidenceStore.Get(key)
		if err != nil {
			evpool.logger.Error("Fastcheck Error", "err", err)
			return false
		}
		var evpb *evproto.Info
		err = evpb.Unmarshal(evBytes)
		if err != nil {
			evpool.logger.Error("Fastcheck Error", "err", err)
			return false
		}
		evInfo, err := InfoFromProto(evpb)
		if err != nil {
			evpool.logger.Error("Fastcheck Error", "err", err)
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

func (evpool *Pool) addPendingEvidence(evInfo *Info) error {
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
		return fmt.Errorf("unable to persist evidence: %w", err)
	}
	evpool.evidenceSize++
	return nil
}

func (evpool *Pool) removePendingEvidence(evidence types.Evidence) {
	key := keyPending(evidence)
	if err := evpool.evidenceStore.Delete(key); err != nil {
		evpool.logger.Error("Unable to delete pending evidence", "err", err)
	} else {
		evpool.logger.Info("Deleted pending evidence", "evidence", evidence)
	}
	evpool.evidenceSize--
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
			evInfo Info
			evpb   evproto.Info
		)
		err := evpb.Unmarshal(val)
		if err != nil {
			return nil, err
		}

		evInfo, err = InfoFromProto(&evpb)
		if err != nil {
			return nil, err
		}

		evidence = append(evidence, evInfo.Evidence)
	}

	return evidence, nil
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
		evBytes := iter.Value()
		var (
			evInfo   Info
			evInfoPb evproto.Info
		)
		err := evInfoPb.Unmarshal(evBytes)
		if err != nil {
			evpool.logger.Error("Unable to unmarshal Evidence", "err", err)
			continue
		}

		evInfo, err = InfoFromProto(&evInfoPb)
		if err != nil {
			evpool.logger.Error("Error in transition evidence from protobuf", "err", err)
			continue
		}
		if !evpool.isExpired(evInfo.Evidence.Height(), evInfo.Time) {
			if len(blockEvidenceMap) != 0 {
				evpool.removeEvidenceFromList(blockEvidenceMap)
			}
			// return the time with which this evidence will have expired so we know when to prune next
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
