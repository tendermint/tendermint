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
	MaxEvidenceBytes int64 = 444

	// An invalid field in the header from LunaticValidatorEvidence.
	// Must be a function of the ABCI application state.
	ValidatorsHashField     = "ValidatorsHash"
	NextValidatorsHashField = "NextValidatorsHash"
	ConsensusHashField      = "ConsensusHash"
	AppHashField            = "AppHash"
	LastResultsHashField    = "LastResultsHash"
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
	VerifyComposite(committedHeader *Header, valSet *ValidatorSet) error
	Split(committedHeader *Header, valSet *ValidatorSet, valToLastHeight map[string]int64) []Evidence
}

func RegisterEvidences(cdc *amino.Codec) {
	cdc.RegisterInterface((*Evidence)(nil), nil)
	cdc.RegisterConcrete(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence", nil)
	cdc.RegisterConcrete(&ConflictingHeadersEvidence{}, "tendermint/ConflictingHeadersEvidence", nil)
	cdc.RegisterConcrete(&PhantomValidatorEvidence{}, "tendermint/PhantomValidatorEvidence", nil)
	cdc.RegisterConcrete(&LunaticValidatorEvidence{}, "tendermint/LunaticValidatorEvidence", nil)
	cdc.RegisterConcrete(&PotentialAmnesiaEvidence{}, "tendermint/PotentialAmnesiaEvidence", nil)
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
	VoteA *Vote
	VoteB *Vote
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(vote1 *Vote, vote2 *Vote) *DuplicateVoteEvidence {
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

// Time returns the time the evidence was created.
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return dve.VoteA.Timestamp
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Address() []byte {
	return dve.VoteA.ValidatorAddress
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

	// Index must be the same
	if dve.VoteA.ValidatorIndex != dve.VoteB.ValidatorIndex {
		return fmt.Errorf(
			"validator indices do not match: %d and %d",
			dve.VoteA.ValidatorIndex,
			dve.VoteB.ValidatorIndex,
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

	// Signatures must be valid
	if !pubKey.VerifyBytes(dve.VoteA.SignBytes(chainID), dve.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifyBytes(dve.VoteB.SignBytes(chainID), dve.VoteB.Signature) {
		return fmt.Errorf("verifying VoteB: %w", ErrVoteInvalidSignature)
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
// observes two conflicting headers, both having 1/3+ of the voting power of
// the currently trusted validator set.
type ConflictingHeadersEvidence struct {
	H1 *SignedHeader `json:"h_1"`
	H2 *SignedHeader `json:"h_2"`
}

var _ Evidence = &ConflictingHeadersEvidence{}
var _ CompositeEvidence = &ConflictingHeadersEvidence{}
var _ Evidence = ConflictingHeadersEvidence{}
var _ CompositeEvidence = ConflictingHeadersEvidence{}

// Split breaks up eviddence into smaller chunks (one per validator except for
// PotentialAmnesiaEvidence): PhantomValidatorEvidence,
// LunaticValidatorEvidence, DuplicateVoteEvidence and
// PotentialAmnesiaEvidence.
//
// committedHeader - header at height H1.Height == H2.Height
// valSet					 - validator set at height H1.Height == H2.Height
// valToLastHeight - map between active validators and respective last heights
func (ev ConflictingHeadersEvidence) Split(committedHeader *Header, valSet *ValidatorSet,
	valToLastHeight map[string]int64) []Evidence {

	evList := make([]Evidence, 0)

	var alternativeHeader *SignedHeader
	if bytes.Equal(committedHeader.Hash(), ev.H1.Hash()) {
		alternativeHeader = ev.H2
	} else {
		alternativeHeader = ev.H1
	}

	// If there are signers(alternativeHeader) that are not part of
	// validators(committedHeader), they misbehaved as they are signing protocol
	// messages in heights they are not validators => immediately slashable
	// (#F4).
	for i, sig := range alternativeHeader.Commit.Signatures {
		if sig.Absent() {
			continue
		}

		lastHeightValidatorWasInSet, ok := valToLastHeight[string(sig.ValidatorAddress)]
		if !ok {
			continue
		}

		if !valSet.HasAddress(sig.ValidatorAddress) {
			evList = append(evList, &PhantomValidatorEvidence{
				Header:                      alternativeHeader.Header,
				Vote:                        alternativeHeader.Commit.GetVote(i),
				LastHeightValidatorWasInSet: lastHeightValidatorWasInSet,
			})
		}
	}

	// If ValidatorsHash, NextValidatorsHash, ConsensusHash, AppHash, and
	// LastResultsHash in alternativeHeader are different (incorrect application
	// state transition), then it is a lunatic misbehavior => immediately
	// slashable (#F5).
	var invalidField string
	switch {
	case !bytes.Equal(committedHeader.ValidatorsHash, alternativeHeader.ValidatorsHash):
		invalidField = "ValidatorsHash"
	case !bytes.Equal(committedHeader.NextValidatorsHash, alternativeHeader.NextValidatorsHash):
		invalidField = "NextValidatorsHash"
	case !bytes.Equal(committedHeader.ConsensusHash, alternativeHeader.ConsensusHash):
		invalidField = "ConsensusHash"
	case !bytes.Equal(committedHeader.AppHash, alternativeHeader.AppHash):
		invalidField = "AppHash"
	case !bytes.Equal(committedHeader.LastResultsHash, alternativeHeader.LastResultsHash):
		invalidField = "LastResultsHash"
	}
	if invalidField != "" {
		for i, sig := range alternativeHeader.Commit.Signatures {
			if sig.Absent() {
				continue
			}
			evList = append(evList, &LunaticValidatorEvidence{
				Header:             alternativeHeader.Header,
				Vote:               alternativeHeader.Commit.GetVote(i),
				InvalidHeaderField: invalidField,
			})
		}
		return evList
	}

	// Use the fact that signatures are sorted by ValidatorAddress.
	var (
		i = 0
		j = 0
	)
OUTER_LOOP:
	for i < len(ev.H1.Commit.Signatures) {
		sigA := ev.H1.Commit.Signatures[i]
		if sigA.Absent() {
			i++
			continue
		}
		// FIXME: Replace with HasAddress once DuplicateVoteEvidence#PubKey is
		// removed.
		_, val := valSet.GetByAddress(sigA.ValidatorAddress)
		if val == nil {
			i++
			continue
		}

		for j < len(ev.H2.Commit.Signatures) {
			sigB := ev.H2.Commit.Signatures[j]
			if sigB.Absent() {
				j++
				continue
			}

			switch bytes.Compare(sigA.ValidatorAddress, sigB.ValidatorAddress) {
			case 0:
				// if H1.Round == H2.Round, and some signers signed different precommit
				// messages in both commits, then it is an equivocation misbehavior =>
				// immediately slashable (#F1).
				if ev.H1.Commit.Round == ev.H2.Commit.Round {
					evList = append(evList, &DuplicateVoteEvidence{
						VoteA: ev.H1.Commit.GetVote(i),
						VoteB: ev.H2.Commit.GetVote(j),
					})
				} else {
					// if H1.Round != H2.Round we need to run full detection procedure => not
					// immediately slashable.
					evList = append(evList, &PotentialAmnesiaEvidence{
						VoteA: ev.H1.Commit.GetVote(i),
						VoteB: ev.H2.Commit.GetVote(j),
					})
				}

				i++
				j++
				continue OUTER_LOOP
			case 1:
				i++
				continue OUTER_LOOP
			case -1:
				j++
			}
		}
	}

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

// VerifyComposite verifies that both headers belong to the same chain, same
// height and signed by 1/3+ of validators at height H1.Height == H2.Height.
func (ev ConflictingHeadersEvidence) VerifyComposite(committedHeader *Header, valSet *ValidatorSet) error {
	var alternativeHeader *SignedHeader
	switch {
	case bytes.Equal(committedHeader.Hash(), ev.H1.Hash()):
		alternativeHeader = ev.H2
	case bytes.Equal(committedHeader.Hash(), ev.H2.Hash()):
		alternativeHeader = ev.H1
	default:
		return errors.New("none of the headers are committed from this node's perspective")
	}

	// ChainID must be the same
	if committedHeader.ChainID != alternativeHeader.ChainID {
		return errors.New("alt header is from a different chain")
	}

	// Height must be the same
	if committedHeader.Height != alternativeHeader.Height {
		return errors.New("alt header is from a different height")
	}

	// Limit the number of signatures to avoid DoS attacks where a header
	// contains too many signatures.
	//
	// Validator set size               = 100 [node]
	// Max validator set size = 100 * 2 = 200 [fork?]
	maxNumValidators := valSet.Size() * 2
	if len(alternativeHeader.Commit.Signatures) > maxNumValidators {
		return errors.Errorf("alt commit contains too many signatures: %d, expected no more than %d",
			len(alternativeHeader.Commit.Signatures),
			maxNumValidators)
	}

	// Header must be signed by at least 1/3+ of voting power of currently
	// trusted validator set.
	if err := valSet.VerifyCommitTrusting(
		alternativeHeader.ChainID,
		alternativeHeader.Commit,
		tmmath.Fraction{Numerator: 1, Denominator: 3}); err != nil {
		return errors.Wrap(err, "alt header does not have 1/3+ of voting power of our validator set")
	}

	return nil
}

func (ev ConflictingHeadersEvidence) Equal(ev2 Evidence) bool {
	switch e2 := ev2.(type) {
	case ConflictingHeadersEvidence:
		return bytes.Equal(ev.H1.Hash(), e2.H1.Hash()) && bytes.Equal(ev.H2.Hash(), e2.H2.Hash())
	case *ConflictingHeadersEvidence:
		return bytes.Equal(ev.H1.Hash(), e2.H1.Hash()) && bytes.Equal(ev.H2.Hash(), e2.H2.Hash())
	default:
		return false
	}
}

func (ev ConflictingHeadersEvidence) ValidateBasic() error {
	if ev.H1 == nil {
		return errors.New("first header is missing")
	}

	if ev.H2 == nil {
		return errors.New("second header is missing")
	}

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

//-------------------------------------------

type PhantomValidatorEvidence struct {
	Header                      *Header `json:"header"`
	Vote                        *Vote   `json:"vote"`
	LastHeightValidatorWasInSet int64   `json:"last_height_validator_was_in_set"`
}

var _ Evidence = &PhantomValidatorEvidence{}
var _ Evidence = PhantomValidatorEvidence{}

func (e PhantomValidatorEvidence) Height() int64 {
	return e.Header.Height
}

func (e PhantomValidatorEvidence) Time() time.Time {
	return e.Header.Time
}

func (e PhantomValidatorEvidence) Address() []byte {
	return e.Vote.ValidatorAddress
}

func (e PhantomValidatorEvidence) Hash() []byte {
	bz := make([]byte, tmhash.Size+crypto.AddressSize)
	copy(bz[:tmhash.Size-1], e.Header.Hash().Bytes())
	copy(bz[tmhash.Size:], e.Vote.ValidatorAddress.Bytes())
	return tmhash.Sum(bz)
}

func (e PhantomValidatorEvidence) Bytes() []byte {
	return cdcEncode(e)
}

func (e PhantomValidatorEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// chainID must be the same
	if chainID != e.Header.ChainID {
		return fmt.Errorf("chainID do not match: %s vs %s",
			chainID,
			e.Header.ChainID,
		)
	}

	if !pubKey.VerifyBytes(e.Vote.SignBytes(chainID), e.Vote.Signature) {
		return errors.New("invalid signature")
	}

	return nil
}

func (e PhantomValidatorEvidence) Equal(ev Evidence) bool {
	switch e2 := ev.(type) {
	case PhantomValidatorEvidence:
		return bytes.Equal(e.Header.Hash(), e2.Header.Hash()) &&
			bytes.Equal(e.Vote.ValidatorAddress, e2.Vote.ValidatorAddress)
	case *PhantomValidatorEvidence:
		return bytes.Equal(e.Header.Hash(), e2.Header.Hash()) &&
			bytes.Equal(e.Vote.ValidatorAddress, e2.Vote.ValidatorAddress)
	default:
		return false
	}
}

func (e PhantomValidatorEvidence) ValidateBasic() error {
	if e.Header == nil {
		return errors.New("empty header")
	}

	if e.Vote == nil {
		return errors.New("empty vote")
	}

	if err := e.Header.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid header: %v", err)
	}

	if err := e.Vote.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid signature: %v", err)
	}

	if !e.Vote.BlockID.IsComplete() {
		return errors.New("expected vote for block")
	}

	if e.Header.Height != e.Vote.Height {
		return fmt.Errorf("header and vote have different heights: %d vs %d",
			e.Header.Height,
			e.Vote.Height,
		)
	}

	if e.LastHeightValidatorWasInSet <= 0 {
		return errors.New("negative or zero LastHeightValidatorWasInSet")
	}

	return nil
}

func (e PhantomValidatorEvidence) String() string {
	return fmt.Sprintf("PhantomValidatorEvidence{%X voted for %d/%X}",
		e.Vote.ValidatorAddress, e.Header.Height, e.Header.Hash())
}

//-------------------------------------------

type LunaticValidatorEvidence struct {
	Header             *Header `json:"header"`
	Vote               *Vote   `json:"vote"`
	InvalidHeaderField string  `json:"invalid_header_field"`
}

var _ Evidence = &LunaticValidatorEvidence{}
var _ Evidence = LunaticValidatorEvidence{}

func (e LunaticValidatorEvidence) Height() int64 {
	return e.Header.Height
}

func (e LunaticValidatorEvidence) Time() time.Time {
	return e.Header.Time
}

func (e LunaticValidatorEvidence) Address() []byte {
	return e.Vote.ValidatorAddress
}

func (e LunaticValidatorEvidence) Hash() []byte {
	bz := make([]byte, tmhash.Size+crypto.AddressSize)
	copy(bz[:tmhash.Size-1], e.Header.Hash().Bytes())
	copy(bz[tmhash.Size:], e.Vote.ValidatorAddress.Bytes())
	return tmhash.Sum(bz)
}

func (e LunaticValidatorEvidence) Bytes() []byte {
	return cdcEncode(e)
}

func (e LunaticValidatorEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// chainID must be the same
	if chainID != e.Header.ChainID {
		return fmt.Errorf("chainID do not match: %s vs %s",
			chainID,
			e.Header.ChainID,
		)
	}

	if !pubKey.VerifyBytes(e.Vote.SignBytes(chainID), e.Vote.Signature) {
		return errors.New("invalid signature")
	}

	return nil
}

func (e LunaticValidatorEvidence) Equal(ev Evidence) bool {
	switch e2 := ev.(type) {
	case LunaticValidatorEvidence:
		return bytes.Equal(e.Header.Hash(), e2.Header.Hash()) &&
			bytes.Equal(e.Vote.ValidatorAddress, e2.Vote.ValidatorAddress)
	case *LunaticValidatorEvidence:
		return bytes.Equal(e.Header.Hash(), e2.Header.Hash()) &&
			bytes.Equal(e.Vote.ValidatorAddress, e2.Vote.ValidatorAddress)
	default:
		return false
	}
}

func (e LunaticValidatorEvidence) ValidateBasic() error {
	if e.Header == nil {
		return errors.New("empty header")
	}

	if e.Vote == nil {
		return errors.New("empty vote")
	}

	if err := e.Header.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid header: %v", err)
	}

	if err := e.Vote.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid signature: %v", err)
	}

	if !e.Vote.BlockID.IsComplete() {
		return errors.New("expected vote for block")
	}

	if e.Header.Height != e.Vote.Height {
		return fmt.Errorf("header and vote have different heights: %d vs %d",
			e.Header.Height,
			e.Vote.Height,
		)
	}

	switch e.InvalidHeaderField {
	case "ValidatorsHash", "NextValidatorsHash", "ConsensusHash", "AppHash", "LastResultsHash":
		return nil
	default:
		return errors.New("unknown invalid header field")
	}
}

func (e LunaticValidatorEvidence) String() string {
	return fmt.Sprintf("LunaticValidatorEvidence{%X voted for %d/%X, which contains invalid %s}",
		e.Vote.ValidatorAddress, e.Header.Height, e.Header.Hash(), e.InvalidHeaderField)
}

func (e LunaticValidatorEvidence) VerifyHeader(committedHeader *Header) error {
	matchErr := func(field string) error {
		return fmt.Errorf("%s matches committed hash", field)
	}

	switch e.InvalidHeaderField {
	case ValidatorsHashField:
		if bytes.Equal(committedHeader.ValidatorsHash, e.Header.ValidatorsHash) {
			return matchErr(ValidatorsHashField)
		}
	case NextValidatorsHashField:
		if bytes.Equal(committedHeader.NextValidatorsHash, e.Header.NextValidatorsHash) {
			return matchErr(NextValidatorsHashField)
		}
	case ConsensusHashField:
		if bytes.Equal(committedHeader.ConsensusHash, e.Header.ConsensusHash) {
			return matchErr(ConsensusHashField)
		}
	case AppHashField:
		if bytes.Equal(committedHeader.AppHash, e.Header.AppHash) {
			return matchErr(AppHashField)
		}
	case LastResultsHashField:
		if bytes.Equal(committedHeader.LastResultsHash, e.Header.LastResultsHash) {
			return matchErr(LastResultsHashField)
		}
	default:
		return errors.New("unknown InvalidHeaderField")
	}

	return nil
}

//-------------------------------------------

type PotentialAmnesiaEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`
}

var _ Evidence = &PotentialAmnesiaEvidence{}
var _ Evidence = PotentialAmnesiaEvidence{}

func (e PotentialAmnesiaEvidence) Height() int64 {
	return e.VoteA.Height
}

func (e PotentialAmnesiaEvidence) Time() time.Time {
	if e.VoteA.Timestamp.Before(e.VoteB.Timestamp) {
		return e.VoteA.Timestamp
	}
	return e.VoteB.Timestamp
}

func (e PotentialAmnesiaEvidence) Address() []byte {
	return e.VoteA.ValidatorAddress
}

func (e PotentialAmnesiaEvidence) Hash() []byte {
	return tmhash.Sum(cdcEncode(e))
}

func (e PotentialAmnesiaEvidence) Bytes() []byte {
	return cdcEncode(e)
}

func (e PotentialAmnesiaEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// pubkey must match address (this should already be true, sanity check)
	addr := e.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}

	// Signatures must be valid
	if !pubKey.VerifyBytes(e.VoteA.SignBytes(chainID), e.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifyBytes(e.VoteB.SignBytes(chainID), e.VoteB.Signature) {
		return fmt.Errorf("verifying VoteB: %w", ErrVoteInvalidSignature)
	}

	return nil
}

func (e PotentialAmnesiaEvidence) Equal(ev Evidence) bool {
	switch e2 := ev.(type) {
	case PotentialAmnesiaEvidence:
		return bytes.Equal(e.Hash(), e2.Hash())
	case *PotentialAmnesiaEvidence:
		return bytes.Equal(e.Hash(), e2.Hash())
	default:
		return false
	}
}

func (e PotentialAmnesiaEvidence) ValidateBasic() error {
	if e.VoteA == nil || e.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", e.VoteA, e.VoteB)
	}
	if err := e.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %v", err)
	}
	if err := e.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %v", err)
	}
	// Enforce Votes are lexicographically sorted on blockID
	if strings.Compare(e.VoteA.BlockID.Key(), e.VoteB.BlockID.Key()) >= 0 {
		return errors.New("amnesia votes in invalid order")
	}

	// H/S must be the same
	if e.VoteA.Height != e.VoteB.Height ||
		e.VoteA.Type != e.VoteB.Type {
		return fmt.Errorf("h/s do not match: %d/%v vs %d/%v",
			e.VoteA.Height, e.VoteA.Type, e.VoteB.Height, e.VoteB.Type)
	}

	// R must be different
	if e.VoteA.Round == e.VoteB.Round {
		return fmt.Errorf("expected votes from different rounds, got %d", e.VoteA.Round)
	}

	// Address must be the same
	if !bytes.Equal(e.VoteA.ValidatorAddress, e.VoteB.ValidatorAddress) {
		return fmt.Errorf("validator addresses do not match: %X vs %X",
			e.VoteA.ValidatorAddress,
			e.VoteB.ValidatorAddress,
		)
	}

	// Index must be the same
	// https://github.com/tendermint/tendermint/issues/4619
	if e.VoteA.ValidatorIndex != e.VoteB.ValidatorIndex {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator indices do not match. Got %d and %d",
			e.VoteA.ValidatorIndex,
			e.VoteB.ValidatorIndex,
		)
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

func (e PotentialAmnesiaEvidence) String() string {
	return fmt.Sprintf("PotentialAmnesiaEvidence{VoteA: %v, VoteB: %v}", e.VoteA, e.VoteB)
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
func NewMockEvidence(height int64, eTime time.Time, address []byte) MockEvidence {
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
