package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Evidence represents any provable malicious activity by a validator.
// Verification logic for each evidence is part of the evidence module.
type Evidence interface {
	Height() int64        // height of the infraction
	Bytes() []byte        // bytes which comprise the evidence
	Hash() []byte         // hash of the evidence
	ValidateBasic() error // basic consistency check
	String() string       // string format of the evidence
}

//--------------------------------------------------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(vote1, vote2 *Vote) *DuplicateVoteEvidence {
	var voteA, voteB *Vote
	if vote1 == nil || vote2 == nil {
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
		VoteA: voteA,
		VoteB: voteB,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("DuplicateVoteEvidence{VoteA: %v, VoteB: %v}", dve.VoteA, dve.VoteB)
}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height
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
		VoteA: voteA,
		VoteB: voteB,
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

	dve := NewDuplicateVoteEvidence(vA, vB)

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
}

var _ Evidence = &LightClientAttackEvidence{}

// Height returns the last height at which the primary provider and witness provider had the same header.
// We use this as the height of the infraction rather than the actual conflicting header because we know
// that the malicious validators were bonded at this height which is important for evidence expiry
func (l *LightClientAttackEvidence) Height() int64 {
	return l.CommonHeight
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

// Hash returns the hash of the header and the commonHeight. This is designed to cause hash collisions with evidence
// that have the same conflicting header and common height but different permutations of validator commit signatures.
// The reason for this is that we don't want to allow several permutations of the same evidence to be committed on
// chain. Ideally we commit the header with the most commit signatures but anything greater than 1/3 is sufficient.
func (l *LightClientAttackEvidence) Hash() []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, l.CommonHeight)
	bz := make([]byte, tmhash.Size+n)
	copy(bz[:tmhash.Size-1], l.ConflictingBlock.Hash().Bytes())
	copy(bz[tmhash.Size:], buf)
	return tmhash.Sum(bz)
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

// String returns a string representation of LightClientAttackEvidence
func (l *LightClientAttackEvidence) String() string {
	return fmt.Sprintf("LightClientAttackEvidence{ConflictingBlock: %v, CommonHeight: %d}",
		l.ConflictingBlock.String(), l.CommonHeight)
}

// ToProto encodes LightClientAttackEvidence to protobuf
func (l *LightClientAttackEvidence) ToProto() (*tmproto.LightClientAttackEvidence, error) {
	conflictingBlock, err := l.ConflictingBlock.ToProto()
	if err != nil {
		return nil, err
	}

	return &tmproto.LightClientAttackEvidence{
		ConflictingBlock: conflictingBlock,
		CommonHeight:     l.CommonHeight,
	}, nil
}

// LightClientAttackEvidenceFromProto decodes protobuf
func LightClientAttackEvidenceFromProto(l *tmproto.LightClientAttackEvidence) (*LightClientAttackEvidence, error) {
	if l == nil {
		return nil, errors.New("empty light client attack evidence")
	}

	conflictingBlock, err := LightBlockFromProto(l.ConflictingBlock)
	if err != nil {
		return nil, err
	}

	le := &LightClientAttackEvidence{
		ConflictingBlock: conflictingBlock,
		CommonHeight:     l.CommonHeight,
	}

	return le, le.ValidateBasic()
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

func NewMockDuplicateVoteEvidenceWithValidator(height int64, time time.Time,
	pv PrivValidator, chainID string) *DuplicateVoteEvidence {
	pubKey, _ := pv.GetPubKey()
	voteA := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vA := voteA.ToProto()
	_ = pv.SignVote(chainID, vA)
	voteA.Signature = vA.Signature
	voteB := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vB := voteB.ToProto()
	_ = pv.SignVote(chainID, vB)
	voteB.Signature = vB.Signature
	return NewDuplicateVoteEvidence(voteA, voteB)
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
