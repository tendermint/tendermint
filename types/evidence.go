package types

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmmath "github.com/tendermint/tendermint/libs/math"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
)

const (
	// MaxEvidenceBytes is a maximum size of any evidence (including amino overhead).
	MaxEvidenceBytes int64 = 484
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
	MaxNum int64
	GotNum int64
}

// NewErrEvidenceOverflow returns a new ErrEvidenceOverflow where got > max.
func NewErrEvidenceOverflow(max, got int64) *ErrEvidenceOverflow {
	return &ErrEvidenceOverflow{max, got}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceOverflow) Error() string {
	return fmt.Sprintf("Too much evidence: Max %d, got %d", err.MaxNum, err.GotNum)
}

//-------------------------------------------

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

type CompositeEvidence interface {
	VerifyComposite(chainID string, valSet *ValidatorSet) error
	Split() []Evidence
}

func RegisterEvidences(cdc *amino.Codec) {
	cdc.RegisterInterface((*Evidence)(nil), nil)
	cdc.RegisterConcrete(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence", nil)
	cdc.RegisterConcrete(&ConflictingHeadersEvidence{}, "tendermint/ConflictingHeadersEvidence", nil)
}

func RegisterMockEvidences(cdc *amino.Codec) {
	cdc.RegisterConcrete(MockEvidence{}, "tendermint/MockEvidence", nil)
	cdc.RegisterConcrete(MockRandomEvidence{}, "tendermint/MockRandomEvidence", nil)
}

const (
	MaxEvidenceBytesDenominator = 10
)

// MaxEvidencePerBlock returns the maximum number of evidences
// allowed in the block and their maximum total size (limitted to 1/10th
// of the maximum block size).
// TODO: change to a constant, or to a fraction of the validator set size.
// See https://github.com/tendermint/tendermint/issues/2590
func MaxEvidencePerBlock(blockMaxBytes int64) (int64, int64) {
	maxBytes := blockMaxBytes / MaxEvidenceBytesDenominator
	maxNum := maxBytes / MaxEvidenceBytes
	return maxNum, maxBytes
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	PubKey crypto.PubKey
	VoteA  *Vote
	VoteB  *Vote
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(pubkey crypto.PubKey, vote1 *Vote, vote2 *Vote) *DuplicateVoteEvidence {
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
		PubKey: pubkey,
		VoteA:  voteA,
		VoteB:  voteB,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("VoteA: %v; VoteB: %v", dve.VoteA, dve.VoteB)

}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height
}

// Time return the time the evidence was created
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return dve.VoteA.Timestamp
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Address() []byte {
	return dve.PubKey.Address()
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Bytes() []byte {
	return cdcEncode(dve)
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() []byte {
	return tmhash.Sum(cdcEncode(dve))
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
		return fmt.Errorf("duplicateVoteEvidence Error: H/R/S does not match. Got %v and %v", dve.VoteA, dve.VoteB)
	}

	// Address must be the same
	if !bytes.Equal(dve.VoteA.ValidatorAddress, dve.VoteB.ValidatorAddress) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator addresses do not match. Got %X and %X",
			dve.VoteA.ValidatorAddress,
			dve.VoteB.ValidatorAddress,
		)
	}

	// Index must be the same
	if dve.VoteA.ValidatorIndex != dve.VoteB.ValidatorIndex {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator indices do not match. Got %d and %d",
			dve.VoteA.ValidatorIndex,
			dve.VoteB.ValidatorIndex,
		)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equals(dve.VoteB.BlockID) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: BlockIDs are the same (%v) - not a real duplicate vote",
			dve.VoteA.BlockID,
		)
	}

	// pubkey must match address (this should already be true, sanity check)
	addr := dve.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("duplicateVoteEvidence FAILED SANITY CHECK - address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}

	// Signatures must be valid
	if !pubKey.VerifyBytes(dve.VoteA.SignBytes(chainID), dve.VoteA.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteA: %v", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifyBytes(dve.VoteB.SignBytes(chainID), dve.VoteB.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteB: %v", ErrVoteInvalidSignature)
	}

	return nil
}

// Equal checks if two pieces of evidence are equal.
func (dve *DuplicateVoteEvidence) Equal(ev Evidence) bool {
	if _, ok := ev.(*DuplicateVoteEvidence); !ok {
		return false
	}

	// just check their hashes
	dveHash := tmhash.Sum(cdcEncode(dve))
	evHash := tmhash.Sum(cdcEncode(ev))
	return bytes.Equal(dveHash, evHash)
}

// ValidateBasic performs basic validation.
func (dve *DuplicateVoteEvidence) ValidateBasic() error {
	if len(dve.PubKey.Bytes()) == 0 {
		return errors.New("empty PubKey")
	}
	if dve.VoteA == nil || dve.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	}
	if err := dve.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %v", err)
	}
	if err := dve.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %v", err)
	}
	// Enforce Votes are lexicographically sorted on blockID
	if strings.Compare(dve.VoteA.BlockID.Key(), dve.VoteB.BlockID.Key()) >= 0 {
		return errors.New("duplicate votes in invalid order")
	}
	return nil
}

//-----------------------------------------------------------------

// UNSTABLE
type MockRandomEvidence struct {
	MockEvidence
	randBytes []byte
}

var _ Evidence = &MockRandomEvidence{}

// UNSTABLE
func NewMockRandomEvidence(height int64, eTime time.Time, address []byte, randBytes []byte) MockRandomEvidence {
	return MockRandomEvidence{
		MockEvidence{
			EvidenceHeight:  height,
			EvidenceTime:    eTime,
			EvidenceAddress: address}, randBytes,
	}
}

func (e MockRandomEvidence) Hash() []byte {
	return []byte(fmt.Sprintf("%d-%x", e.EvidenceHeight, e.randBytes))
}

// UNSTABLE
type MockEvidence struct {
	EvidenceHeight  int64
	EvidenceTime    time.Time
	EvidenceAddress []byte
}

var _ Evidence = &MockEvidence{}

// UNSTABLE
func NewMockEvidence(height int64, eTime time.Time, idx int, address []byte) MockEvidence {
	return MockEvidence{
		EvidenceHeight:  height,
		EvidenceTime:    eTime,
		EvidenceAddress: address}
}

func (e MockEvidence) Height() int64   { return e.EvidenceHeight }
func (e MockEvidence) Time() time.Time { return e.EvidenceTime }
func (e MockEvidence) Address() []byte { return e.EvidenceAddress }
func (e MockEvidence) Hash() []byte {
	return []byte(fmt.Sprintf("%d-%x-%s",
		e.EvidenceHeight, e.EvidenceAddress, e.EvidenceTime))
}
func (e MockEvidence) Bytes() []byte {
	return []byte(fmt.Sprintf("%d-%x-%s",
		e.EvidenceHeight, e.EvidenceAddress, e.EvidenceTime))
}
func (e MockEvidence) Verify(chainID string, pubKey crypto.PubKey) error { return nil }
func (e MockEvidence) Equal(ev Evidence) bool {
	e2 := ev.(MockEvidence)
	return e.EvidenceHeight == e2.EvidenceHeight &&
		bytes.Equal(e.EvidenceAddress, e2.EvidenceAddress)
}
func (e MockEvidence) ValidateBasic() error { return nil }
func (e MockEvidence) String() string {
	return fmt.Sprintf("Evidence: %d/%s/%s", e.EvidenceHeight, e.Time(), e.EvidenceAddress)
}

//-------------------------------------------

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
	return merkle.SimpleHashFromByteSlices(evidenceBzs)
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

//-------------------------------------------

// ConflictingHeadersEvidence is primarily used by the light client when it
// observes two (or more) conflicting headers, both having 1/3+ of the voting
// power of the currently trusted validator set.
type ConflictingHeadersEvidence struct {
	H1 SignedHeader `json:"h_1"`
	H2 SignedHeader `json:"h_2"`
}

func (ev ConflictingHeadersEvidence) Split(committedHeader *Header, valSet *ValidatorSet) []Evidence {
	evList := make([]Evidence, 0)

	// maliciousHeaders := make([]SignedHeader, 0)
	// switch {
	// case bytes.Equal(ev.H1.Hash(), committedHeader.Hash()):
	// case bytes.Equal(ev.H2.Hash(), committedHeader.Hash()):
	// default:

	// }

	// if there are signers(H2) that are not part of validators(H1), they
	// misbehaved as they are signing protocol messages in heights they are not
	// validators => immediately slashable (#F4).

	// if H1.Round == H2.Round, and some signers signed different precommit
	// messages in both commits, then it is an equivocation misbehavior =>
	// immediately slashable (#F1).

	// if H1.Round != H2.Round we need to run full detection procedure => not
	// immediately slashable.

	// if ValidatorsHash, NextValidatorsHash, ConsensusHash, AppHash, and
	// LastResultsHash in H2 are different (incorrect application state
	// transition), then it is a lunatic misbehavior => immediately slashable
	// (#F5).

	return evList
}

func (ev ConflictingHeadersEvidence) Height() int64 { return ev.H1.Height }

// XXX: this is not the time of equivocation
func (ev ConflictingHeadersEvidence) Time() time.Time { return ev.H1.Time }

func (ev ConflictingHeadersEvidence) Address() []byte {
	panic("use ConflictingHeadersEvidence#Split to split evidence into individual pieces")
}

func (ev ConflictingHeadersEvidence) Bytes() []byte {
	return cdcEncode(ev)
}

func (ev ConflictingHeadersEvidence) Hash() []byte {
	bz := make([]byte, tmhash.Size*2)
	copy(bz[:tmhash.Size-1], ev.H1.Hash().Bytes())
	copy(bz[tmhash.Size:], ev.H2.Hash().Bytes())
	return tmhash.Sum(bz)
}

func (ev ConflictingHeadersEvidence) Verify(chainID string, _ crypto.PubKey) error {
	panic("use ConflictingHeadersEvidence#VerifyComposite to verify composite evidence")
}

func (ev ConflictingHeadersEvidence) VerifyComposite(chainID string, valSet *ValidatorSet) error {
	// ChainID must be the same
	if ev.H1.ChainID != ev.H2.ChainID {
		return errors.New("headers are from different chains")
	}
	if chainID != ev.H1.ChainID {
		return errors.New("header #1 is from a different chain")
	}

	// Height must be the same
	if ev.H1.Height != ev.H2.Height {
		return errors.New("headers are from different heights")
	}

	// Check signatures.
	if len(ev.H1.Commit.Signatures) != valSet.Size() {
		return errors.Errorf("commit #1 contains too many signatures: %d, expected %d",
			len(ev.H1.Commit.Signatures),
			valSet.Size())
	}
	if len(ev.H2.Commit.Signatures) != valSet.Size() {
		return errors.Errorf("commit #2 contains too many signatures: %d, expected %d",
			len(ev.H2.Commit.Signatures),
			valSet.Size())
	}

	// Check both headers are signed by 1/3+ of voting power.
	if err := valSet.VerifyCommitTrusting(chainID, ev.H1.Commit.BlockID, ev.H1.Height,
		ev.H1.Commit, tmmath.Fraction{Numerator: 1, Denominator: 3}); err != nil {
		return errors.Wrap(err, "failed to verify H1")
	}
	if err := valSet.VerifyCommitTrusting(chainID, ev.H1.Commit.BlockID, ev.H1.Height,
		ev.H1.Commit, tmmath.Fraction{Numerator: 1, Denominator: 3}); err != nil {
		return errors.Wrap(err, "failed to verify H2")
	}

	return nil
}

func (ev ConflictingHeadersEvidence) Equal(ev2 Evidence) bool {
	ev2T, ok := ev2.(ConflictingHeadersEvidence)
	if !ok {
		return false
	}
	return bytes.Equal(ev.H1.Hash(), ev2T.H1.Hash()) && bytes.Equal(ev.H2.Hash(), ev2T.H2.Hash())
}

func (ev ConflictingHeadersEvidence) ValidateBasic() error {
	if err := ev.H1.ValidateBasic(ev.H1.ChainID); err != nil {
		return fmt.Errorf("h1: %w", err)
	}
	if err := ev.H2.ValidateBasic(ev.H2.ChainID); err != nil {
		return fmt.Errorf("h2: %w", err)
	}
	return nil
}

func (ev ConflictingHeadersEvidence) String() string {
	return fmt.Sprintf("ConflictingHeadersEvidence{H1: %d#%X, H2: %d#%X}",
		ev.H1.Height, ev.H1.Hash(),
		ev.H2.Height, ev.H2.Hash())
}

type PhantomValidatorEvidence struct {
	Header    Header    `json:"header"`
	CommitSig CommitSig `json:"commit_sig"`
}

var _ Evidence = &PhantomValidatorEvidence{}

func (e PhantomValidatorEvidence) Height() int64 {
	return e.Header.Height
}

func (e PhantomValidatorEvidence) Time() time.Time {
	return e.Header.Time
}

func (e PhantomValidatorEvidence) Address() []byte {
	return e.CommitSig.ValidatorAddress
}

func (e PhantomValidatorEvidence) Hash() []byte {
	bz := make([]byte, tmhash.Size*2)
	copy(bz[:tmhash.Size-1], e.Header.Hash().Bytes())
	copy(bz[tmhash.Size:], e.CommitSig.ValidatorAddress.Bytes())
	return tmhash.Sum(bz)
}

func (e PhantomValidatorEvidence) Bytes() []byte {
	return cdcEncode(e)
}

func (e PhantomValidatorEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	return nil
}

func (e PhantomValidatorEvidence) Equal(ev Evidence) bool {
	e2 := ev.(PhantomValidatorEvidence)
	return e.Header.Height == e2.Header.Height &&
		bytes.Equal(e.CommitSig.ValidatorAddress, e2.CommitSig.ValidatorAddress)
}

func (e PhantomValidatorEvidence) ValidateBasic() error { return nil }

func (e PhantomValidatorEvidence) String() string {
	return fmt.Sprintf("PhantomValidatorEvidence{%X voted for %d/%X}", e.Header.Height, e.Header.Hash(), e.CommitSig.ValidatorAddress)
}
