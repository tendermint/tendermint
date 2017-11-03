package types

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tmlibs/merkle"
)

// ErrEvidenceInvalid wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrEvidenceInvalid struct {
	Evidence   Evidence
	ErrorValue error
}

func NewEvidenceInvalidErr(ev Evidence, err error) *ErrEvidenceInvalid {
	return &ErrEvidenceInvalid{ev, err}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceInvalid) Error() string {
	return fmt.Sprintf("Invalid evidence: %v. Evidence: %v", err.ErrorValue, err.Evidence)
}

//-------------------------------------------

type HistoricalValidators interface {
	LoadValidators(height int) *ValidatorSet
}

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int                                            // height of the equivocation
	Address() []byte                                        // address of the equivocating validator
	Index() int                                             // index of the validator in the validator set
	Hash() []byte                                           // hash of the evidence
	Verify(chainID string, vals HistoricalValidators) error // verify the evidence
	Equal(Evidence) bool                                    // check equality of evidence

	String() string
}

//-------------------------------------------

//EvidenceSet is a thread-safe set of evidence.
type EvidenceSet struct {
	sync.RWMutex
	evidences evidences
}

//Evidence returns a copy of all the evidence.
func (evset EvidenceSet) Evidence() []Evidence {
	evset.RLock()
	defer evset.RUnlock()
	evCopy := make([]Evidence, len(evset.evidences))
	for i, ev := range evset.evidences {
		evCopy[i] = ev
	}
	return evCopy
}

// Size returns the number of pieces of evidence in the set.
func (evset EvidenceSet) Size() int {
	evset.RLock()
	defer evset.RUnlock()
	return len(evset.evidences)
}

// Hash returns a merkle hash of the evidence.
func (evset EvidenceSet) Hash() []byte {
	evset.RLock()
	defer evset.RUnlock()
	return evset.evidences.Hash()
}

// Has returns true if the given evidence is in the set.
func (evset EvidenceSet) Has(evidence Evidence) bool {
	evset.RLock()
	defer evset.RUnlock()
	return evset.evidences.Has(evidence)
}

// String returns a string representation of the evidence.
func (evset EvidenceSet) String() string {
	evset.RLock()
	defer evset.RUnlock()
	return evset.evidences.String()
}

// Add adds the given evidence to the set.
// TODO: and persists it to disk.
func (evset EvidenceSet) Add(evidence Evidence) {
	evset.Lock()
	defer evset.Unlock()
	evset.evidences = append(evset.evidences, evidence)
}

// Reset empties the evidence set.
func (evset EvidenceSet) Reset() {
	evset.Lock()
	defer evset.Unlock()
	evset.evidences = make(evidences, 0)

}

//-------------------------------------------

type evidences []Evidence

func (evs evidences) Hash() []byte {
	// Recursive impl.
	// Copied from tmlibs/merkle to avoid allocations
	switch len(evs) {
	case 0:
		return nil
	case 1:
		return evs[0].Hash()
	default:
		left := evidences(evs[:(len(evs)+1)/2]).Hash()
		right := evidences(evs[(len(evs)+1)/2:]).Hash()
		return merkle.SimpleHashFromTwoHashes(left, right)
	}
}

func (evs evidences) String() string {
	s := ""
	for _, e := range evs {
		s += fmt.Sprintf("%s\t\t", e)
	}
	return s
}

func (evs evidences) Has(evidence Evidence) bool {
	for _, ev := range evs {
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting votes.
type DuplicateVoteEvidence struct {
	PubKey crypto.PubKey
	VoteA  *Vote
	VoteB  *Vote
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("VoteA: %v; VoteB: %v", dve.VoteA, dve.VoteB)

}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int {
	return dve.VoteA.Height
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Address() []byte {
	return dve.PubKey.Address()
}

// Index returns the index of the validator.
func (dve *DuplicateVoteEvidence) Index() int {
	return dve.VoteA.ValidatorIndex
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() []byte {
	return merkle.SimpleHashFromBinary(dve)
}

// Verify returns an error if the two votes aren't conflicting.
// To be conflicting, they must be from the same validator, for the same H/R/S, but for different blocks.
func (dve *DuplicateVoteEvidence) Verify(chainID string, vals HistoricalValidators) error {

	// TODO: verify (cs.Height - dve.Height) < MaxHeightDiff

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
	// XXX: Should we enforce index is the same ?
	if dve.VoteA.ValidatorIndex != dve.VoteB.ValidatorIndex {
		return fmt.Errorf("DuplicateVoteEvidence Error: Validator indices do not match. Got %d and %d", dve.VoteA.ValidatorIndex, dve.VoteB.ValidatorIndex)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equals(dve.VoteB.BlockID) {
		return fmt.Errorf("DuplicateVoteEvidence Error: BlockIDs are the same (%v) - not a real duplicate vote!", dve.VoteA.BlockID)
	}

	// Signatures must be valid
	if !dve.PubKey.VerifyBytes(SignBytes(chainID, dve.VoteA), dve.VoteA.Signature) {
		return ErrVoteInvalidSignature
	}
	if !dve.PubKey.VerifyBytes(SignBytes(chainID, dve.VoteB), dve.VoteB.Signature) {
		return ErrVoteInvalidSignature
	}

	// The address must have been an active validator at the height
	height := dve.Height()
	addr := dve.Address()
	idx := dve.Index()
	valset := vals.LoadValidators(height)
	valIdx, val := valset.GetByAddress(addr)
	if val == nil {
		return fmt.Errorf("Address %X was not a validator at height %d", addr, height)
	} else if idx != valIdx {
		return fmt.Errorf("Address %X was validator %d at height %d, not %d", addr, valIdx, height, idx)
	}

	return nil
}

// Equal checks if two pieces of evidence are equal.
func (dve *DuplicateVoteEvidence) Equal(ev Evidence) bool {
	if _, ok := ev.(*DuplicateVoteEvidence); !ok {
		return false
	}

	// just check their hashes
	return bytes.Equal(merkle.SimpleHashFromBinary(dve), merkle.SimpleHashFromBinary(ev))
}
