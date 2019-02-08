package types

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"

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

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int64                                     // height of the equivocation
	Address() []byte                                   // address of the equivocating validator
	Bytes() []byte                                     // bytes which compromise the evidence
	Hash() []byte                                      // hash of the evidence
	Verify(chainID string, pubKey crypto.PubKey) error // verify the evidence
	Equal(Evidence) bool                               // check equality of evidence

	ValidateBasic() error
	String() string
}

func RegisterEvidences(cdc *amino.Codec) {
	cdc.RegisterInterface((*Evidence)(nil), nil)
	cdc.RegisterConcrete(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence", nil)
}

func RegisterMockEvidences(cdc *amino.Codec) {
	cdc.RegisterConcrete(MockGoodEvidence{}, "tendermint/MockGoodEvidence", nil)
	cdc.RegisterConcrete(MockRandomGoodEvidence{}, "tendermint/MockRandomGoodEvidence", nil)
	cdc.RegisterConcrete(MockBadEvidence{}, "tendermint/MockBadEvidence", nil)
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

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("VoteA: %v; VoteB: %v", dve.VoteA, dve.VoteB)

}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height
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
// To be conflicting, they must be from the same validator, for the same H/R/S, but for different blocks.
func (dve *DuplicateVoteEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// H/R/S must be the same
	if dve.VoteA.Height != dve.VoteB.Height ||
		dve.VoteA.Round != dve.VoteB.Round ||
		dve.VoteA.Type != dve.VoteB.Type {
		return fmt.Errorf("DuplicateVoteEvidence Error: H/R/S does not match. Got %v and %v", dve.VoteA, dve.VoteB)
	}

	// Address must be the same
	if !bytes.Equal(dve.VoteA.ValidatorAddress, dve.VoteB.ValidatorAddress) {
		return fmt.Errorf("DuplicateVoteEvidence Error: Validator addresses do not match. Got %X and %X", dve.VoteA.ValidatorAddress, dve.VoteB.ValidatorAddress)
	}

	// Index must be the same
	if dve.VoteA.ValidatorIndex != dve.VoteB.ValidatorIndex {
		return fmt.Errorf("DuplicateVoteEvidence Error: Validator indices do not match. Got %d and %d", dve.VoteA.ValidatorIndex, dve.VoteB.ValidatorIndex)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equals(dve.VoteB.BlockID) {
		return fmt.Errorf("DuplicateVoteEvidence Error: BlockIDs are the same (%v) - not a real duplicate vote", dve.VoteA.BlockID)
	}

	// pubkey must match address (this should already be true, sanity check)
	addr := dve.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("DuplicateVoteEvidence FAILED SANITY CHECK - address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}

	// Signatures must be valid
	if !pubKey.VerifyBytes(dve.VoteA.SignBytes(chainID), dve.VoteA.Signature) {
		return fmt.Errorf("DuplicateVoteEvidence Error verifying VoteA: %v", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifyBytes(dve.VoteB.SignBytes(chainID), dve.VoteB.Signature) {
		return fmt.Errorf("DuplicateVoteEvidence Error verifying VoteB: %v", ErrVoteInvalidSignature)
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
		return errors.New("Empty PubKey")
	}
	if dve.VoteA == nil || dve.VoteB == nil {
		return fmt.Errorf("One or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	}
	if err := dve.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("Invalid VoteA: %v", err)
	}
	if err := dve.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("Invalid VoteB: %v", err)
	}
	return nil
}

//-----------------------------------------------------------------

// UNSTABLE
type MockRandomGoodEvidence struct {
	MockGoodEvidence
	randBytes []byte
}

var _ Evidence = &MockRandomGoodEvidence{}

// UNSTABLE
func NewMockRandomGoodEvidence(height int64, address []byte, randBytes []byte) MockRandomGoodEvidence {
	return MockRandomGoodEvidence{
		MockGoodEvidence{height, address}, randBytes,
	}
}

func (e MockRandomGoodEvidence) Hash() []byte {
	return []byte(fmt.Sprintf("%d-%x", e.Height_, e.randBytes))
}

// UNSTABLE
type MockGoodEvidence struct {
	Height_  int64
	Address_ []byte
}

var _ Evidence = &MockGoodEvidence{}

// UNSTABLE
func NewMockGoodEvidence(height int64, idx int, address []byte) MockGoodEvidence {
	return MockGoodEvidence{height, address}
}

func (e MockGoodEvidence) Height() int64   { return e.Height_ }
func (e MockGoodEvidence) Address() []byte { return e.Address_ }
func (e MockGoodEvidence) Hash() []byte {
	return []byte(fmt.Sprintf("%d-%x", e.Height_, e.Address_))
}
func (e MockGoodEvidence) Bytes() []byte {
	return []byte(fmt.Sprintf("%d-%x", e.Height_, e.Address_))
}
func (e MockGoodEvidence) Verify(chainID string, pubKey crypto.PubKey) error { return nil }
func (e MockGoodEvidence) Equal(ev Evidence) bool {
	e2 := ev.(MockGoodEvidence)
	return e.Height_ == e2.Height_ &&
		bytes.Equal(e.Address_, e2.Address_)
}
func (e MockGoodEvidence) ValidateBasic() error { return nil }
func (e MockGoodEvidence) String() string {
	return fmt.Sprintf("GoodEvidence: %d/%s", e.Height_, e.Address_)
}

// UNSTABLE
type MockBadEvidence struct {
	MockGoodEvidence
}

func (e MockBadEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	return fmt.Errorf("MockBadEvidence")
}
func (e MockBadEvidence) Equal(ev Evidence) bool {
	e2 := ev.(MockBadEvidence)
	return e.Height_ == e2.Height_ &&
		bytes.Equal(e.Address_, e2.Address_)
}
func (e MockBadEvidence) ValidateBasic() error { return nil }
func (e MockBadEvidence) String() string {
	return fmt.Sprintf("BadEvidence: %d/%s", e.Height_, e.Address_)
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
