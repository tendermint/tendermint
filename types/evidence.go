package types

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/crypto/tmhash"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
)

const (
	// MaxEvidenceBytes is a maximum size of any evidence (including amino overhead).
	MaxEvidenceBytes int64 = 476
)

// ErrEvidenceInvalid wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrEvidenceInvalid struct {
	Evidence   EvidenceI
	ErrorValue error
}

// NewErrEvidenceInvalid returns a new EvidenceInvalid with the given err.
func NewErrEvidenceInvalid(ev EvidenceI, err error) *ErrEvidenceInvalid {
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

// Hash returns the hash of the data.
func (data *EvidenceData) Hash() tmbytes.HexBytes {
	if data.hash == nil {
		evidenceBzs := make([][]byte, len(data.Evidence))
		for i := 0; i < len(data.Evidence); i++ {
			evidenceI := data.Evidence[i].GetEvidenceI()
			evidenceBzs[i] = evidenceI.Bytes()
		}
		data.hash = merkle.SimpleHashFromByteSlices(evidenceBzs)
	}
	return data.hash
}

// StringIndented returns a string representation of the evidence.
func (data *EvidenceData) StringIndented(indent string) string {
	if data == nil {
		return "nil-Evidence"
	}
	evStrings := make([]string, tmmath.MinInt(len(data.Evidence), 21))
	for i, ev := range data.Evidence {
		if i == 20 {
			evStrings[i] = fmt.Sprintf("... (%v total)", len(data.Evidence))
			break
		}
		evStrings[i] = fmt.Sprintf("Evidence:%v", ev)
	}
	return fmt.Sprintf(`EvidenceData{
%s  %v
%s}#%v`,
		indent, strings.Join(evStrings, "\n"+indent+"  "),
		indent, data.hash)
}

//-------------------------------------------

// Evidence represents any provable malicious activity by a validator
type EvidenceI interface {
	Height() int64                                     // height of the equivocation
	Time() time.Time                                   // time of the equivocation
	Address() []byte                                   // address of the equivocating validator
	Bytes() []byte                                     // bytes which comprise the evidence
	Hash() []byte                                      // hash of the evidence
	Verify(chainID string, pubKey crypto.PubKey) error // verify the evidence
	Equal(EvidenceI) bool                              // check equality of evidence

	ValidateBasic() error
	String() string
}

func RegisterEvidences(cdc *amino.Codec) {
	cdc.RegisterInterface((*EvidenceI)(nil), nil)
	cdc.RegisterConcrete(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence", nil)
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

var _ EvidenceI = &DuplicateVoteEvidence{}

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
	return cdcEncode(dve) //TODO: change to []byte
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
func (dve *DuplicateVoteEvidence) Equal(ev EvidenceI) bool {
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

var _ EvidenceI = &MockRandomEvidence{}

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

var _ EvidenceI = &MockEvidence{}

// UNSTABLE
func NewMockEvidence(height int64, eTime time.Time, idx int32, address []byte) MockEvidence {
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
func (e MockEvidence) Equal(ev EvidenceI) bool {
	e2 := ev.(MockEvidence)
	return e.EvidenceHeight == e2.EvidenceHeight &&
		bytes.Equal(e.EvidenceAddress, e2.EvidenceAddress)
}
func (e MockEvidence) ValidateBasic() error { return nil }
func (e MockEvidence) String() string {
	return fmt.Sprintf("Evidence: %d/%s/%s", e.EvidenceHeight, e.Time(), e.EvidenceAddress)
}

//-------------------------------------------

//TODO: deprecated (remove)
// EvidenceList is a list of Evidence. Evidences is not a word.
type EvidenceList []EvidenceI

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
func (evl EvidenceList) Has(evidence EvidenceI) bool {
	for _, ev := range evl {
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
}
