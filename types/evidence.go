package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Evidence represents any provable malicious activity by a validator.
type Evidence interface {
	Height() int64                                     // height of the equivocation
	Time() time.Time                                   // time of the equivocation
	Address() []byte                                   // address of the equivocating validator
	Bytes() []byte                                     // bytes which comprise the evidence
	Hash() []byte                                      // hash of the evidence
	Verify(chainID string, pubKey crypto.PubKey) error // verify the evidence
	Equal(Evidence) bool                               // check equality of evidence

	ValidateBasic() error
	String() string
}

const (
	// MaxEvidenceBytes is a maximum size of any evidence (including amino overhead).
	MaxEvidenceBytes int64 = 444
)

// ErrEvidenceInvalid wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrEvidenceInvalid struct {
	Evidence   Evidence
	ErrorValue error
}

// NewErrEvidenceInvalid returns a new EvidenceInvalid with the given err.
func NewErrEvidenceInvalid(ev Evidence, err error) *ErrEvidenceInvalid {
	return &ErrEvidenceInvalid{ev, err}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceInvalid) Error() string {
	return fmt.Sprintf("Invalid evidence: %v. Evidence: %v", err.ErrorValue, err.Evidence)
}

// ErrEvidenceOverflow is for when there is too much evidence in a block.
type ErrEvidenceOverflow struct {
	MaxNum int
	GotNum int
}

// NewErrEvidenceOverflow returns a new ErrEvidenceOverflow where got > max.
func NewErrEvidenceOverflow(max, got int) *ErrEvidenceOverflow {
	return &ErrEvidenceOverflow{max, got}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceOverflow) Error() string {
	return fmt.Sprintf("Too much evidence: Max %d, got %d", err.MaxNum, err.GotNum)
}

//-------------------------------------------

func EvidenceToProto(evidence Evidence) (*tmproto.Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.(type) {
	case *DuplicateVoteEvidence:
		pbevi := evi.ToProto()
		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_DuplicateVoteEvidence{
				DuplicateVoteEvidence: pbevi,
			},
		}
		return tp, nil

	case *PotentialAmnesiaEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_PotentialAmnesiaEvidence{
				PotentialAmnesiaEvidence: pbevi,
			},
		}

		return tp, nil

	case *AmnesiaEvidence:
		aepb := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_AmnesiaEvidence{
				AmnesiaEvidence: aepb,
			},
		}

		return tp, nil
	default:
		return nil, fmt.Errorf("toproto: evidence is not recognized: %T", evi)
	}
}

func EvidenceFromProto(evidence *tmproto.Evidence) (Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.Sum.(type) {
	case *tmproto.Evidence_DuplicateVoteEvidence:
		return DuplicateVoteEvidenceFromProto(evi.DuplicateVoteEvidence)
	case *tmproto.Evidence_PotentialAmnesiaEvidence:
		return PotentialAmnesiaEvidenceFromProto(evi.PotentialAmnesiaEvidence)
	case *tmproto.Evidence_AmnesiaEvidence:
		return AmnesiaEvidenceFromProto(evi.AmnesiaEvidence)
	default:
		return nil, errors.New("evidence is not recognized")
	}
}

func init() {
	tmjson.RegisterType(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence")
	tmjson.RegisterType(&PotentialAmnesiaEvidence{}, "tendermint/PotentialAmnesiaEvidence")
	tmjson.RegisterType(&AmnesiaEvidence{}, "tendermint/AmnesiaEvidence")
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`

	Timestamp time.Time `json:"timestamp"`
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(vote1, vote2 *Vote, time time.Time) *DuplicateVoteEvidence {
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

		Timestamp: time,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("DuplicateVoteEvidence{VoteA: %v, VoteB: %v, Time: %v}", dve.VoteA, dve.VoteB, dve.Timestamp)
}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height
}

// Time returns time of the latest vote.
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return dve.Timestamp
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Address() []byte {
	return dve.VoteA.ValidatorAddress
}

// Hash returns the hash of the evidence.
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
	pbe := dve.ToProto()
	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return tmhash.Sum(bz)
}

// Verify returns an error if the two votes aren't conflicting.
//
// To be conflicting, they must be from the same validator, for the same H/R/S,
// but for different blocks.
func (dve *DuplicateVoteEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// H/R/S must be the same
	if dve.VoteA.Height != dve.VoteB.Height ||
		dve.VoteA.Round != dve.VoteB.Round ||
		dve.VoteA.Type != dve.VoteB.Type {
		return fmt.Errorf("h/r/s does not match: %d/%d/%v vs %d/%d/%v",
			dve.VoteA.Height, dve.VoteA.Round, dve.VoteA.Type,
			dve.VoteB.Height, dve.VoteB.Round, dve.VoteB.Type)
	}

	// Address must be the same
	if !bytes.Equal(dve.VoteA.ValidatorAddress, dve.VoteB.ValidatorAddress) {
		return fmt.Errorf("validator addresses do not match: %X vs %X",
			dve.VoteA.ValidatorAddress,
			dve.VoteB.ValidatorAddress,
		)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equals(dve.VoteB.BlockID) {
		return fmt.Errorf(
			"block IDs are the same (%v) - not a real duplicate vote",
			dve.VoteA.BlockID,
		)
	}

	// pubkey must match address (this should already be true, sanity check)
	addr := dve.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}
	va := dve.VoteA.ToProto()
	vb := dve.VoteB.ToProto()
	// Signatures must be valid
	if !pubKey.VerifySignature(VoteSignBytes(chainID, va), dve.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifySignature(VoteSignBytes(chainID, vb), dve.VoteB.Signature) {
		return fmt.Errorf("verifying VoteB: %w", ErrVoteInvalidSignature)
	}

	return nil
}

// Equal checks if two pieces of evidence are equal.
func (dve *DuplicateVoteEvidence) Equal(ev Evidence) bool {
	if _, ok := ev.(*DuplicateVoteEvidence); !ok {
		return false
	}
	pbdev := dve.ToProto()
	bz, err := pbdev.Marshal()
	if err != nil {
		panic(err)
	}

	var evbz []byte
	if ev, ok := ev.(*DuplicateVoteEvidence); ok {
		evpb := ev.ToProto()
		evbz, err = evpb.Marshal()
		if err != nil {
			panic(err)
		}
	}

	// just check their hashes
	dveHash := tmhash.Sum(bz)
	evHash := tmhash.Sum(evbz)
	return bytes.Equal(dveHash, evHash)
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

func (dve *DuplicateVoteEvidence) ToProto() *tmproto.DuplicateVoteEvidence {
	voteB := dve.VoteB.ToProto()
	voteA := dve.VoteA.ToProto()
	tp := tmproto.DuplicateVoteEvidence{
		VoteA:     voteA,
		VoteB:     voteB,
		Timestamp: dve.Timestamp,
	}
	return &tp
}

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

	dve := NewDuplicateVoteEvidence(vA, vB, pb.Timestamp)

	return dve, dve.ValidateBasic()
}

//-------------------------------------------

// PotentialAmnesiaEvidence is constructed when a validator votes on two different blocks at different rounds
// in the same height. PotentialAmnesiaEvidence can then evolve into AmnesiaEvidence if the indicted validator
// is incapable of providing the proof of lock change that validates voting twice in the allotted trial period.
// Heightstamp is used for each node to keep a track of how much time has passed so as to know when the trial period
// is finished and is set when the node first receives the evidence. Votes are ordered based on their timestamp
type PotentialAmnesiaEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`

	HeightStamp int64
	Timestamp   time.Time `json:"timestamp"`
}

var _ Evidence = &PotentialAmnesiaEvidence{}

// NewPotentialAmnesiaEvidence creates a new instance of the evidence and orders the votes correctly
func NewPotentialAmnesiaEvidence(voteA, voteB *Vote, time time.Time) *PotentialAmnesiaEvidence {
	if voteA == nil || voteB == nil {
		return nil
	}

	if voteA.Timestamp.Before(voteB.Timestamp) {
		return &PotentialAmnesiaEvidence{VoteA: voteA, VoteB: voteB, Timestamp: time}
	}
	return &PotentialAmnesiaEvidence{VoteA: voteB, VoteB: voteA, Timestamp: time}
}

func (e *PotentialAmnesiaEvidence) Height() int64 {
	return e.VoteA.Height
}

func (e *PotentialAmnesiaEvidence) Time() time.Time {
	return e.Timestamp
}

func (e *PotentialAmnesiaEvidence) Address() []byte {
	return e.VoteA.ValidatorAddress
}

// NOTE: Heightstamp must not be included in hash
func (e *PotentialAmnesiaEvidence) Hash() []byte {
	v1, err := e.VoteA.ToProto().Marshal()
	if err != nil {
		panic(fmt.Errorf("trying to hash potential amnesia evidence, err: %w", err))
	}

	v2, err := e.VoteB.ToProto().Marshal()
	if err != nil {
		panic(fmt.Errorf("trying to hash potential amnesia evidence, err: %w", err))
	}

	return tmhash.Sum(append(v1, v2...))
}

func (e *PotentialAmnesiaEvidence) Bytes() []byte {
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (e *PotentialAmnesiaEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// pubkey must match address (this should already be true, sanity check)
	addr := e.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}

	va := e.VoteA.ToProto()
	vb := e.VoteB.ToProto()

	// Signatures must be valid
	if !pubKey.VerifySignature(VoteSignBytes(chainID, va), e.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifySignature(VoteSignBytes(chainID, vb), e.VoteB.Signature) {
		return fmt.Errorf("verifying VoteB: %w", ErrVoteInvalidSignature)
	}

	return nil
}

func (e *PotentialAmnesiaEvidence) Equal(ev Evidence) bool {
	if e2, ok := ev.(*PotentialAmnesiaEvidence); ok {
		return e.Height() == e2.Height() && e.VoteA.Round == e2.VoteA.Round && e.VoteB.Round == e2.VoteB.Round &&
			bytes.Equal(e.Address(), e2.Address())
	}
	return false
}

func (e *PotentialAmnesiaEvidence) ValidateBasic() error {
	if e == nil {
		return errors.New("empty potential amnesia evidence")
	}

	if e.VoteA == nil || e.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", e.VoteA, e.VoteB)
	}

	if err := e.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %v", err)
	}
	if err := e.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %v", err)
	}

	// H/S must be the same
	if e.VoteA.Height != e.VoteB.Height {
		return fmt.Errorf("heights do not match: %d vs %d",
			e.VoteA.Height, e.VoteB.Height)
	}

	if e.VoteA.Round == e.VoteB.Round {
		return fmt.Errorf("votes must be for different rounds: %d",
			e.VoteA.Round)
	}

	// Enforce that vote A came before vote B
	if e.VoteA.Timestamp.After(e.VoteB.Timestamp) {
		return fmt.Errorf("vote A should have a timestamp before vote B, but got %s > %s",
			e.VoteA.Timestamp, e.VoteB.Timestamp)
	}

	// Address must be the same
	if !bytes.Equal(e.VoteA.ValidatorAddress, e.VoteB.ValidatorAddress) {
		return fmt.Errorf("validator addresses do not match: %X vs %X",
			e.VoteA.ValidatorAddress,
			e.VoteB.ValidatorAddress,
		)
	}

	if e.VoteA.BlockID.IsZero() {
		return errors.New("first vote is for a nil block - voter hasn't locked on a block")
	}

	// BlockIDs must be different
	if e.VoteA.BlockID.Equals(e.VoteB.BlockID) {
		return fmt.Errorf(
			"block IDs are the same (%v) - not a real duplicate vote",
			e.VoteA.BlockID,
		)
	}

	return nil
}

func (e *PotentialAmnesiaEvidence) String() string {
	return fmt.Sprintf("PotentialAmnesiaEvidence{VoteA: %v, VoteB: %v}", e.VoteA, e.VoteB)
}

// Primed finds whether the PotentialAmnesiaEvidence is ready to be upgraded to Amnesia Evidence. It is decided if
// either the prosecuted node voted in the past or if the allocated trial period has expired without a proof of lock
// change having been provided.
func (e *PotentialAmnesiaEvidence) Primed(trialPeriod, currentHeight int64) bool {
	// voted in the past can be instantly punishable
	if e.VoteA.Round > e.VoteB.Round {
		return true
	}
	// has the trial period expired
	if e.HeightStamp > 0 {
		return e.HeightStamp+trialPeriod <= currentHeight
	}
	return false
}

func (e *PotentialAmnesiaEvidence) ToProto() *tmproto.PotentialAmnesiaEvidence {
	voteB := e.VoteB.ToProto()
	voteA := e.VoteA.ToProto()

	tp := &tmproto.PotentialAmnesiaEvidence{
		VoteA:       voteA,
		VoteB:       voteB,
		HeightStamp: e.HeightStamp,
		Timestamp:   e.Timestamp,
	}

	return tp
}

// ------------------

// ProofOfLockChange (POLC) proves that a node followed the consensus protocol and voted for a precommit in two
// different rounds because the node received a majority of votes for a different block in the latter round. In cases of
// amnesia evidence, a suspected node will need ProofOfLockChange to prove that the node did not break protocol.
type ProofOfLockChange struct {
	Votes  []*Vote       `json:"votes"`
	PubKey crypto.PubKey `json:"pubkey"`
}

// MakePOLCFromVoteSet can be used when a majority of prevotes or precommits for a block is seen
// that the node has itself not yet voted for in order to process the vote set into a proof of lock change
func NewPOLCFromVoteSet(voteSet *VoteSet, pubKey crypto.PubKey, blockID BlockID) (*ProofOfLockChange, error) {
	polc := newPOLCFromVoteSet(voteSet, pubKey, blockID)
	return polc, polc.ValidateBasic()
}

func newPOLCFromVoteSet(voteSet *VoteSet, pubKey crypto.PubKey, blockID BlockID) *ProofOfLockChange {
	if voteSet == nil {
		return nil
	}
	var votes []*Vote
	valSetSize := voteSet.Size()
	for valIdx := int32(0); int(valIdx) < valSetSize; valIdx++ {
		vote := voteSet.GetByIndex(valIdx)
		if vote != nil && vote.BlockID.Equals(blockID) {
			votes = append(votes, vote)
		}
	}
	return NewPOLC(votes, pubKey)
}

// NewPOLC creates a POLC
func NewPOLC(votes []*Vote, pubKey crypto.PubKey) *ProofOfLockChange {
	return &ProofOfLockChange{
		Votes:  votes,
		PubKey: pubKey,
	}
}

// EmptyPOLC returns an empty polc. This is used when no polc has been provided in the allocated trial period time
// and the node now needs to move to upgrading to AmnesiaEvidence and hence uses an empty polc
func NewEmptyPOLC() *ProofOfLockChange {
	return &ProofOfLockChange{
		nil,
		nil,
	}
}

func (e *ProofOfLockChange) Height() int64 {
	return e.Votes[0].Height
}

// Time returns time of the latest vote.
func (e *ProofOfLockChange) Time() time.Time {
	latest := e.Votes[0].Timestamp
	for _, vote := range e.Votes {
		if vote.Timestamp.After(latest) {
			latest = vote.Timestamp
		}
	}
	return latest
}

func (e *ProofOfLockChange) Round() int32 {
	return e.Votes[0].Round
}

func (e *ProofOfLockChange) Address() []byte {
	return e.PubKey.Address()
}

func (e *ProofOfLockChange) BlockID() BlockID {
	return e.Votes[0].BlockID
}

// ValidateVotes checks the polc against the validator set of that height. The function makes sure that the polc
// contains a majority of votes and that each
func (e *ProofOfLockChange) ValidateVotes(valSet *ValidatorSet, chainID string) error {
	if e.IsAbsent() {
		return errors.New("polc is empty")
	}
	talliedVotingPower := int64(0)
	votingPowerNeeded := valSet.TotalVotingPower() * 2 / 3
	for _, vote := range e.Votes {
		exists := false
		for _, validator := range valSet.Validators {
			if bytes.Equal(validator.Address, vote.ValidatorAddress) {
				exists = true
				v := vote.ToProto()
				if !validator.PubKey.VerifySignature(VoteSignBytes(chainID, v), vote.Signature) {
					return fmt.Errorf("cannot verify vote (from validator: %d) against signature: %v",
						vote.ValidatorIndex, vote.Signature)
				}

				talliedVotingPower += validator.VotingPower
			}
		}
		if !exists {
			return fmt.Errorf("vote was not from a validator in this set: %v", vote.String())
		}
	}
	if talliedVotingPower <= votingPowerNeeded {
		return ErrNotEnoughVotingPowerSigned{
			Got:    talliedVotingPower,
			Needed: votingPowerNeeded + 1,
		}
	}
	return nil
}

func (e *ProofOfLockChange) Equal(e2 *ProofOfLockChange) bool {
	return bytes.Equal(e.Address(), e2.Address()) && e.Height() == e2.Height() &&
		e.Round() == e2.Round()
}

func (e *ProofOfLockChange) ValidateBasic() error {
	if e == nil {
		return errors.New("empty proof of lock change")
	}

	// first check if the polc is absent / empty
	if e.IsAbsent() {
		return nil
	}

	if e.PubKey == nil {
		return errors.New("missing public key")
	}
	// validate basic doesn't count the number of votes and their voting power, this is to be done by VerifyEvidence
	if e.Votes == nil || len(e.Votes) == 0 {
		return errors.New("missing votes")
	}
	// height, round and vote type must be the same for all votes
	height := e.Height()
	round := e.Round()
	if round == 0 {
		return errors.New("can't have a polc for the first round")
	}
	voteType := e.Votes[0].Type
	for idx, vote := range e.Votes {
		if vote == nil {
			return fmt.Errorf("nil vote at index: %d", idx)
		}

		if err := vote.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid vote#%d: %w", idx, err)
		}

		if vote.Height != height {
			return fmt.Errorf("invalid height for vote#%d: %d instead of %d", idx, vote.Height, height)
		}

		if vote.Round != round {
			return fmt.Errorf("invalid round for vote#%d: %d instead of %d", idx, vote.Round, round)
		}

		if vote.Type != voteType {
			return fmt.Errorf("invalid vote type for vote#%d: %d instead of %d", idx, vote.Type, voteType)
		}

		if !vote.BlockID.Equals(e.BlockID()) {
			return fmt.Errorf("vote must be for the same block id: %v instead of %v", e.BlockID(), vote.BlockID)
		}

		if bytes.Equal(vote.ValidatorAddress.Bytes(), e.PubKey.Address().Bytes()) {
			return fmt.Errorf("vote validator address cannot be the same as the public key address: %X all votes %v",
				vote.ValidatorAddress.Bytes(), e.PubKey.Address().Bytes())
		}

		for i := idx + 1; i < len(e.Votes); i++ {
			if bytes.Equal(vote.ValidatorAddress.Bytes(), e.Votes[i].ValidatorAddress.Bytes()) {
				return fmt.Errorf("duplicate votes: %v", vote)
			}
		}

	}
	return nil
}

func (e *ProofOfLockChange) String() string {
	if e.IsAbsent() {
		return "Empty ProofOfLockChange"
	}
	return fmt.Sprintf("ProofOfLockChange {Address: %X, Height: %d, Round: %d", e.Address(), e.Height(),
		e.Votes[0].Round)
}

// IsAbsent checks if the polc is empty
func (e *ProofOfLockChange) IsAbsent() bool {
	return e.Votes == nil && e.PubKey == nil
}

func (e *ProofOfLockChange) ToProto() (*tmproto.ProofOfLockChange, error) {
	plc := new(tmproto.ProofOfLockChange)
	vpb := make([]*tmproto.Vote, len(e.Votes))

	// if absent create empty proto polc
	if e.IsAbsent() {
		return plc, nil
	}

	if e.Votes == nil {
		return plc, errors.New("invalid proof of lock change (no votes), did you forget to validate?")
	}
	for i, v := range e.Votes {
		pb := v.ToProto()
		if pb != nil {
			vpb[i] = pb
		}
	}

	pk, err := cryptoenc.PubKeyToProto(e.PubKey)
	if err != nil {
		return plc, fmt.Errorf("invalid proof of lock change (err: %w), did you forget to validate?", err)
	}
	plc.PubKey = &pk
	plc.Votes = vpb

	return plc, nil
}

// AmnesiaEvidence is the progression of PotentialAmnesiaEvidence and is used to prove an infringement of the
// Tendermint consensus when a validator incorrectly sends a vote in a later round without correctly changing the lock
type AmnesiaEvidence struct {
	*PotentialAmnesiaEvidence
	Polc *ProofOfLockChange
}

// Height, Time, Address, and Verify, and Hash functions are all inherited by the PotentialAmnesiaEvidence struct
var _ Evidence = &AmnesiaEvidence{}

func NewAmnesiaEvidence(pe *PotentialAmnesiaEvidence, proof *ProofOfLockChange) *AmnesiaEvidence {
	return &AmnesiaEvidence{
		pe,
		proof,
	}
}

// Note: Amnesia evidence with or without a polc are considered the same
func (e *AmnesiaEvidence) Equal(ev Evidence) bool {
	if e2, ok := ev.(*AmnesiaEvidence); ok {
		return e.PotentialAmnesiaEvidence.Equal(e2.PotentialAmnesiaEvidence)
	}
	return false
}

func (e *AmnesiaEvidence) Bytes() []byte {
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(fmt.Errorf("converting amnesia evidence to bytes, err: %w", err))
	}

	return bz
}

func (e *AmnesiaEvidence) ValidateBasic() error {
	if e == nil {
		return errors.New("empty amnesia evidence")
	}
	if e.Polc == nil || e.PotentialAmnesiaEvidence == nil {
		return errors.New("amnesia evidence is missing either the polc or the potential amnesia evidence")
	}

	if err := e.PotentialAmnesiaEvidence.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid potential amnesia evidence: %w", err)
	}
	if !e.Polc.IsAbsent() {
		if err := e.Polc.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid proof of lock change: %w", err)
		}

		if !bytes.Equal(e.PotentialAmnesiaEvidence.Address(), e.Polc.Address()) {
			return fmt.Errorf("validator addresses do not match (%X - %X)", e.PotentialAmnesiaEvidence.Address(),
				e.Polc.Address())
		}

		if e.PotentialAmnesiaEvidence.Height() != e.Polc.Height() {
			return fmt.Errorf("heights do not match (%d - %d)", e.PotentialAmnesiaEvidence.Height(),
				e.Polc.Height())
		}

		if e.Polc.Round() <= e.VoteA.Round || e.Polc.Round() > e.VoteB.Round {
			return fmt.Errorf("polc must be between %d and %d (inclusive)", e.VoteA.Round+1, e.VoteB.Round)
		}

		if !e.Polc.BlockID().Equals(e.PotentialAmnesiaEvidence.VoteB.BlockID) && !e.Polc.BlockID().IsZero() {
			return fmt.Errorf("polc must be either for a nil block or for the same block as the second vote: %v != %v",
				e.Polc.BlockID(), e.PotentialAmnesiaEvidence.VoteB.BlockID)
		}

		if e.Polc.Time().After(e.PotentialAmnesiaEvidence.VoteB.Timestamp) {
			return fmt.Errorf("validator voted again before receiving a majority of votes for the new block: %v is after %v",
				e.Polc.Time(), e.PotentialAmnesiaEvidence.VoteB.Timestamp)
		}
	}
	return nil
}

// ViolatedConsensus assess on the basis of the AmnesiaEvidence whether the validator has violated the
// Tendermint consensus. Evidence must be validated first (see ValidateBasic).
// We are only interested in proving that the latter of the votes in terms of time was correctly done.
func (e *AmnesiaEvidence) ViolatedConsensus() (bool, string) {
	// a validator having voted cannot go back and vote on an earlier round
	if e.PotentialAmnesiaEvidence.VoteA.Round > e.PotentialAmnesiaEvidence.VoteB.Round {
		return true, "validator went back and voted on a previous round"
	}

	// if empty, then no proof was provided to defend the validators actions
	if e.Polc.IsAbsent() {
		return true, "no proof of lock was provided"
	}

	return false, ""
}

func (e *AmnesiaEvidence) String() string {
	return fmt.Sprintf("AmnesiaEvidence{ %v, polc: %v }", e.PotentialAmnesiaEvidence, e.Polc)
}

func (e *AmnesiaEvidence) ToProto() *tmproto.AmnesiaEvidence {
	paepb := e.PotentialAmnesiaEvidence.ToProto()

	polc, err := e.Polc.ToProto()
	if err != nil {
		polc, _ = NewEmptyPOLC().ToProto()
	}

	return &tmproto.AmnesiaEvidence{
		PotentialAmnesiaEvidence: paepb,
		Polc:                     polc,
	}
}

func ProofOfLockChangeFromProto(pb *tmproto.ProofOfLockChange) (*ProofOfLockChange, error) {
	if pb == nil {
		return nil, errors.New("nil proof of lock change")
	}

	plc := new(ProofOfLockChange)

	// check if it is an empty polc
	if pb.PubKey == nil && pb.Votes == nil {
		return plc, nil
	}

	if pb.Votes == nil {
		return nil, errors.New("proofOfLockChange: is not absent but has no votes")
	}

	vpb := make([]*Vote, len(pb.Votes))
	for i, v := range pb.Votes {
		vi, err := VoteFromProto(v)
		if err != nil {
			return nil, err
		}
		vpb[i] = vi
	}

	if pb.PubKey == nil {
		return nil, errors.New("proofOfLockChange: is not absent but has nil PubKey")
	}
	pk, err := cryptoenc.PubKeyFromProto(*pb.PubKey)
	if err != nil {
		return nil, err
	}

	plc.PubKey = pk
	plc.Votes = vpb

	return plc, plc.ValidateBasic()
}

func PotentialAmnesiaEvidenceFromProto(pb *tmproto.PotentialAmnesiaEvidence) (*PotentialAmnesiaEvidence, error) {
	voteA, err := VoteFromProto(pb.GetVoteA())
	if err != nil {
		return nil, err
	}

	voteB, err := VoteFromProto(pb.GetVoteB())
	if err != nil {
		return nil, err
	}
	tp := PotentialAmnesiaEvidence{
		VoteA:       voteA,
		VoteB:       voteB,
		HeightStamp: pb.GetHeightStamp(),
		Timestamp:   pb.Timestamp,
	}

	return &tp, tp.ValidateBasic()
}

func AmnesiaEvidenceFromProto(pb *tmproto.AmnesiaEvidence) (*AmnesiaEvidence, error) {
	if pb == nil {
		return nil, errors.New("nil amnesia evidence")
	}

	pae, err := PotentialAmnesiaEvidenceFromProto(pb.PotentialAmnesiaEvidence)
	if err != nil {
		return nil, fmt.Errorf("decoding to amnesia evidence, err: %w", err)
	}
	polc, err := ProofOfLockChangeFromProto(pb.Polc)
	if err != nil {
		return nil, fmt.Errorf("decoding to amnesia evidence, err: %w", err)
	}

	tp := &AmnesiaEvidence{
		PotentialAmnesiaEvidence: pae,
		Polc:                     polc,
	}

	return tp, tp.ValidateBasic()
}

//--------------------------------------------------

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
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
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
	return NewDuplicateVoteEvidence(voteA, voteB, time)
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

// mock polc - fails validate basic, not stable
func NewMockPOLC(height int64, time time.Time, pubKey crypto.PubKey) ProofOfLockChange {
	voteVal := NewMockPV()
	pKey, _ := voteVal.GetPubKey()
	vote := Vote{Type: tmproto.PrecommitType, Height: height, Round: 1, BlockID: BlockID{},
		Timestamp: time, ValidatorAddress: pKey.Address(), ValidatorIndex: 1, Signature: []byte{}}

	v := vote.ToProto()
	if err := voteVal.SignVote("mock-chain-id", v); err != nil {
		panic(err)
	}
	vote.Signature = v.Signature

	return ProofOfLockChange{
		Votes:  []*Vote{&vote},
		PubKey: pubKey,
	}
}
