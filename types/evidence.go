package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Evidence represents any provable malicious activity by a validator.
// Verification logic for each evidence is part of the evidence module.
type Evidence interface {
	ABCI() []abci.Evidence // forms individual evidence to be sent to the application
	Bytes() []byte         // bytes which comprise the evidence
	Hash() []byte          // hash of the evidence
	Height() int64         // height of the infraction
	String() string        // string format of the evidence
	Time() time.Time       // time of the infraction
	ValidateBasic() error  // basic consistency check
}

//--------------------------------------------------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`

	// abci specific information
	TotalVotingPower int64
	ValidatorPower   int64
	Timestamp        time.Time
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(vote1, vote2 *Vote, blockTime time.Time, valSet *ValidatorSet) *DuplicateVoteEvidence {
	var voteA, voteB *Vote
	if vote1 == nil || vote2 == nil || valSet == nil {
		return nil
	}
	idx, val := valSet.GetByAddress(vote1.ValidatorAddress)
	if idx == -1 {
		return nil
	}

	if strings.Compare(vote1.BlockID.Key(), vote2.BlockID.Key()) == -1 {
		voteA = vote1
		voteB = vote2
	} else {
		voteA = vote2
		voteB = vote1
	}
	return &DuplicateVoteEvidence{
		VoteA:            voteA,
		VoteB:            voteB,
		TotalVotingPower: valSet.TotalVotingPower(),
		ValidatorPower:   val.VotingPower,
		Timestamp:        blockTime,
	}
}

// ABCI returns the application relevant representation of the evidence
func (dve *DuplicateVoteEvidence) ABCI() []abci.Evidence {
	return []abci.Evidence{{
		Type: abci.EvidenceType_DUPLICATE_VOTE,
		Validator: abci.Validator{
			Address: dve.VoteA.ValidatorAddress,
			Power:   dve.ValidatorPower,
		},
		Height:           dve.VoteA.Height,
		Time:             dve.Timestamp,
		TotalVotingPower: dve.TotalVotingPower,
	}}
}

// Bytes returns the proto-encoded evidence as a byte array.
func (dve *DuplicateVoteEvidence) Bytes() []byte {
	pbe := dve.ToProto()
	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() []byte {
	return tmhash.Sum(dve.Bytes())
}

// Height returns the height of the infraction
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("DuplicateVoteEvidence{VoteA: %v, VoteB: %v}", dve.VoteA, dve.VoteB)
}

// Time returns the time of the infraction
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return dve.Timestamp
}

// ValidateBasic performs basic validation.
func (dve *DuplicateVoteEvidence) ValidateBasic() error {
	if dve == nil {
		return errors.New("empty duplicate vote evidence")
	}

	if dve.VoteA == nil || dve.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	}
	if err := dve.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %w", err)
	}
	if err := dve.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %w", err)
	}
	// Enforce Votes are lexicographically sorted on blockID
	if strings.Compare(dve.VoteA.BlockID.Key(), dve.VoteB.BlockID.Key()) >= 0 {
		return errors.New("duplicate votes in invalid order")
	}
	return nil
}

// ToProto encodes DuplicateVoteEvidence to protobuf
func (dve *DuplicateVoteEvidence) ToProto() *tmproto.DuplicateVoteEvidence {
	voteB := dve.VoteB.ToProto()
	voteA := dve.VoteA.ToProto()
	tp := tmproto.DuplicateVoteEvidence{
		VoteA:            voteA,
		VoteB:            voteB,
		TotalVotingPower: dve.TotalVotingPower,
		ValidatorPower:   dve.ValidatorPower,
		Timestamp:        dve.Timestamp,
	}
	return &tp
}

// DuplicateVoteEvidenceFromProto decodes protobuf into DuplicateVoteEvidence
func DuplicateVoteEvidenceFromProto(pb *tmproto.DuplicateVoteEvidence) (*DuplicateVoteEvidence, error) {
	if pb == nil {
		return nil, errors.New("nil duplicate vote evidence")
	}

	vA, err := VoteFromProto(pb.VoteA)
	if err != nil {
		return nil, err
	}

	vB, err := VoteFromProto(pb.VoteB)
	if err != nil {
		return nil, err
	}

	dve := &DuplicateVoteEvidence{
		VoteA:            vA,
		VoteB:            vB,
		TotalVotingPower: pb.TotalVotingPower,
		ValidatorPower:   pb.ValidatorPower,
		Timestamp:        pb.Timestamp,
	}

	return dve, dve.ValidateBasic()
}

//------------------------------------ LIGHT EVIDENCE --------------------------------------

// LightClientAttackEvidence is a generalized evidence that captures all forms of known attacks on
// a light client such that a full node can verify, propose and commit the evidence on-chain for
// punishment of the malicious validators. There are three forms of attacks: Lunatic, Equivocation
// and Amnesia. These attacks are exhaustive. You can find a more detailed overview of this at
// tendermint/docs/architecture/adr-047-handling-evidence-from-light-client.md
type LightClientAttackEvidence struct {
	ConflictingBlock *LightBlock
	CommonHeight     int64

	// abci specific information
	ByzantineValidators []Validator
	TotalVotingPower    int64
	Timestamp           time.Time
}

var _ Evidence = &LightClientAttackEvidence{}

// ABCI forms an array of abci evidence for each byzantine validator
func (l *LightClientAttackEvidence) ABCI() []abci.Evidence {
	abciEv := make([]abci.Evidence, len(l.ByzantineValidators))
	for idx, byzVal := range l.ByzantineValidators {
		abciEv[idx] = abci.Evidence{
			Type:             abci.EvidenceType_LIGHT_CLIENT_ATTACK,
			Validator:        TM2PB.Validator(&byzVal),
			Height:           l.Height(),
			Time:             l.Timestamp,
			TotalVotingPower: l.TotalVotingPower,
		}
	}
	return abciEv
}

// Bytes returns the proto-encoded evidence as a byte array
func (l *LightClientAttackEvidence) Bytes() []byte {
	pbe, err := l.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}
	return bz
}

// Hash returns the hash of the header and the commonHeight. This is designed to cause hash collisions
// with evidence that have the same conflicting header and common height but different permutations
// of validator commit signatures. The reason for this is that we don't want to allow several
// permutations of the same evidence to be committed on chain. Ideally we commit the header with the
// most commit signatures (captures the most byzantine validators) but anything greater than 1/3 is sufficient.
func (l *LightClientAttackEvidence) Hash() []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, l.CommonHeight)
	bz := make([]byte, tmhash.Size+n)
	copy(bz[:tmhash.Size-1], l.ConflictingBlock.Hash().Bytes())
	copy(bz[tmhash.Size:], buf)
	return tmhash.Sum(bz)
}

// Height returns the last height at which the primary provider and witness provider had the same header.
// We use this as the height of the infraction rather than the actual conflicting header because we know
// that the malicious validators were bonded at this height which is important for evidence expiry
func (l *LightClientAttackEvidence) Height() int64 {
	return l.CommonHeight
}

// String returns a string representation of LightClientAttackEvidence
func (l *LightClientAttackEvidence) String() string {
	return fmt.Sprintf("LightClientAttackEvidence{ConflictingBlock: %v, CommonHeight: %d}",
		l.ConflictingBlock.String(), l.CommonHeight)
}

// Time returns the time of the common block where the infraction leveraged off.
func (l *LightClientAttackEvidence) Time() time.Time {
	return l.Timestamp
}

// ValidateBasic performs basic validation such that the evidence is consistent and can now be used for verification.
func (l *LightClientAttackEvidence) ValidateBasic() error {
	if l.ConflictingBlock == nil {
		return errors.New("conflicting block is nil")
	}

	// this check needs to be done before we can run validate basic
	if l.ConflictingBlock.Header == nil {
		return errors.New("conflicting block missing header")
	}

	if err := l.ConflictingBlock.ValidateBasic(l.ConflictingBlock.ChainID); err != nil {
		return fmt.Errorf("invalid conflicting light block: %w", err)
	}

	if l.CommonHeight <= 0 {
		return errors.New("negative or zero common height")
	}

	// check that common height isn't ahead of the height of the conflicting block. It
	// is possible that they are the same height if the light node witnesses either an
	// amnesia or a equivocation attack.
	if l.CommonHeight > l.ConflictingBlock.Height {
		return fmt.Errorf("common height is ahead of the conflicting block height (%d > %d)",
			l.CommonHeight, l.ConflictingBlock.Height)
	}

	return nil
}

// ToProto encodes LightClientAttackEvidence to protobuf
func (l *LightClientAttackEvidence) ToProto() (*tmproto.LightClientAttackEvidence, error) {
	conflictingBlock, err := l.ConflictingBlock.ToProto()
	if err != nil {
		return nil, err
	}

	byzVals := make([]tmproto.Validator, len(l.ByzantineValidators))
	for idx, val := range l.ByzantineValidators {
		valpb, err := val.ToProto()
		if err != nil {
			return nil, err
		}
		byzVals[idx] = *valpb
	}

	return &tmproto.LightClientAttackEvidence{
		ConflictingBlock:    conflictingBlock,
		CommonHeight:        l.CommonHeight,
		ByzantineValidators: byzVals,
		TotalVotingPower:    l.TotalVotingPower,
		Timestamp:           l.Timestamp,
	}, nil
}

// LightClientAttackEvidenceFromProto decodes protobuf
func LightClientAttackEvidenceFromProto(lpb *tmproto.LightClientAttackEvidence) (*LightClientAttackEvidence, error) {
	if lpb == nil {
		return nil, errors.New("empty light client attack evidence")
	}

	conflictingBlock, err := LightBlockFromProto(lpb.ConflictingBlock)
	if err != nil {
		return nil, err
	}

	byzVals := make([]Validator, len(lpb.ByzantineValidators))
	for idx, valpb := range lpb.ByzantineValidators {
		val, err := ValidatorFromProto(&valpb)
		if err != nil {
			return nil, err
		}
		byzVals[idx] = *val
	}

	l := &LightClientAttackEvidence{
		ConflictingBlock:    conflictingBlock,
		CommonHeight:        lpb.CommonHeight,
		ByzantineValidators: byzVals,
		TotalVotingPower:    lpb.TotalVotingPower,
		Timestamp:           lpb.Timestamp,
	}

	return l, l.ValidateBasic()
}

//------------------------------------------------------------------------------------------

// EvidenceList is a list of Evidence. Evidences is not a word.
type EvidenceList []Evidence

// Hash returns the simple merkle root hash of the EvidenceList.
func (evl EvidenceList) Hash() []byte {
	// These allocations are required because Evidence is not of type Bytes, and
	// golang slices can't be typed cast. This shouldn't be a performance problem since
	// the Evidence size is capped.
	evidenceBzs := make([][]byte, len(evl))
	for i := 0; i < len(evl); i++ {
		evidenceBzs[i] = evl[i].Bytes()
	}
	return merkle.HashFromByteSlices(evidenceBzs)
}

func (evl EvidenceList) String() string {
	s := ""
	for _, e := range evl {
		s += fmt.Sprintf("%s\t\t", e)
	}
	return s
}

// Has returns true if the evidence is in the EvidenceList.
func (evl EvidenceList) Has(evidence Evidence) bool {
	for _, ev := range evl {
		if bytes.Equal(evidence.Hash(), ev.Hash()) {
			return true
		}
	}
	return false
}

//------------------------------------------ PROTO --------------------------------------

// EvidenceToProto is a generalized function for encoding evidence that conforms to the
// evidence interface to protobuf
func EvidenceToProto(evidence Evidence) (*tmproto.Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.(type) {
	case *DuplicateVoteEvidence:
		pbev := evi.ToProto()
		return &tmproto.Evidence{
			Sum: &tmproto.Evidence_DuplicateVoteEvidence{
				DuplicateVoteEvidence: pbev,
			},
		}, nil

	case *LightClientAttackEvidence:
		pbev, err := evi.ToProto()
		if err != nil {
			return nil, err
		}
		return &tmproto.Evidence{
			Sum: &tmproto.Evidence_LightClientAttackEvidence{
				LightClientAttackEvidence: pbev,
			},
		}, nil

	default:
		return nil, fmt.Errorf("toproto: evidence is not recognized: %T", evi)
	}
}

// EvidenceFromProto is a generalized function for decoding protobuf into the
// evidence interface
func EvidenceFromProto(evidence *tmproto.Evidence) (Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.Sum.(type) {
	case *tmproto.Evidence_DuplicateVoteEvidence:
		return DuplicateVoteEvidenceFromProto(evi.DuplicateVoteEvidence)
	case *tmproto.Evidence_LightClientAttackEvidence:
		return LightClientAttackEvidenceFromProto(evi.LightClientAttackEvidence)
	default:
		return nil, errors.New("evidence is not recognized")
	}
}

func init() {
	tmjson.RegisterType(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence")
	tmjson.RegisterType(&LightClientAttackEvidence{}, "tendermint/LightClientAttackEvidence")
}

//-------------------------------------------- ERRORS --------------------------------------

// ErrInvalidEvidence wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrInvalidEvidence struct {
	Evidence Evidence
	Reason   error
}

// NewErrInvalidEvidence returns a new EvidenceInvalid with the given err.
func NewErrInvalidEvidence(ev Evidence, err error) *ErrInvalidEvidence {
	return &ErrInvalidEvidence{ev, err}
}

// Error returns a string representation of the error.
func (err *ErrInvalidEvidence) Error() string {
	return fmt.Sprintf("Invalid evidence: %v. Evidence: %v", err.Reason, err.Evidence)
}

// ErrEvidenceOverflow is for when there the amount of evidence exceeds the max bytes.
type ErrEvidenceOverflow struct {
	Max int64
	Got int64
}

// NewErrEvidenceOverflow returns a new ErrEvidenceOverflow where got > max.
func NewErrEvidenceOverflow(max, got int64) *ErrEvidenceOverflow {
	return &ErrEvidenceOverflow{max, got}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceOverflow) Error() string {
	return fmt.Sprintf("Too much evidence: Max %d, got %d", err.Max, err.Got)
}

//-------------------------------------------- MOCKING --------------------------------------

// unstable - use only for testing

// assumes the round to be 0 and the validator index to be 0
func NewMockDuplicateVoteEvidence(height int64, time time.Time, chainID string) *DuplicateVoteEvidence {
	val := NewMockPV()
	return NewMockDuplicateVoteEvidenceWithValidator(height, time, val, chainID)
}

// assumes voting power to be 10 and validator to be the only one in the set
func NewMockDuplicateVoteEvidenceWithValidator(height int64, time time.Time,
	pv PrivValidator, chainID string) *DuplicateVoteEvidence {
	pubKey, _ := pv.GetPubKey()
	val := NewValidator(pubKey, 10)
	voteA := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vA := voteA.ToProto()
	_ = pv.SignVote(chainID, vA)
	voteA.Signature = vA.Signature
	voteB := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vB := voteB.ToProto()
	_ = pv.SignVote(chainID, vB)
	voteB.Signature = vB.Signature
	return NewDuplicateVoteEvidence(voteA, voteB, time, NewValidatorSet([]*Validator{val}))
}

func makeMockVote(height int64, round, index int32, addr Address,
	blockID BlockID, time time.Time) *Vote {
	return &Vote{
		Type:             tmproto.SignedMsgType(2),
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        time,
		ValidatorAddress: addr,
		ValidatorIndex:   index,
	}
}

func randBlockID() BlockID {
	return BlockID{
		Hash: tmrand.Bytes(tmhash.Size),
		PartSetHeader: PartSetHeader{
			Total: 1,
			Hash:  tmrand.Bytes(tmhash.Size),
		},
	}
}
