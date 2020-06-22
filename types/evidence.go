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
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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

func EvidenceToProto(evidence Evidence) (*tmproto.Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.(type) {
	case *DuplicateVoteEvidence:
		pbevi := evi.ToProto()
		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_DuplicateVoteEvidence{
				DuplicateVoteEvidence: &pbevi,
			},
		}
		return tp, nil
	case ConflictingHeadersEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_ConflictingHeadersEvidence{
				ConflictingHeadersEvidence: &pbevi,
			},
		}

		return tp, nil
	case *ConflictingHeadersEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_ConflictingHeadersEvidence{
				ConflictingHeadersEvidence: &pbevi,
			},
		}

		return tp, nil
	case *LunaticValidatorEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_LunaticValidatorEvidence{
				LunaticValidatorEvidence: &pbevi,
			},
		}

		return tp, nil
	case LunaticValidatorEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_LunaticValidatorEvidence{
				LunaticValidatorEvidence: &pbevi,
			},
		}

		return tp, nil
	case *PhantomValidatorEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_PhantomValidatorEvidence{
				PhantomValidatorEvidence: &pbevi,
			},
		}

		return tp, nil
	case PhantomValidatorEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_PhantomValidatorEvidence{
				PhantomValidatorEvidence: &pbevi,
			},
		}

		return tp, nil
	case *PotentialAmnesiaEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_PotentialAmnesiaEvidence{
				PotentialAmnesiaEvidence: &pbevi,
			},
		}

		return tp, nil
	case PotentialAmnesiaEvidence:
		pbevi := evi.ToProto()

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_PotentialAmnesiaEvidence{
				PotentialAmnesiaEvidence: &pbevi,
			},
		}
		return tp, nil

	case AmnesiaEvidence:
		return AmnesiaEvidenceToProto(evi)

	case *AmnesiaEvidence:
		return AmnesiaEvidenceToProto(*evi)

	case MockEvidence:
		if err := evi.ValidateBasic(); err != nil {
			return nil, err
		}

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_MockEvidence{
				MockEvidence: &tmproto.MockEvidence{
					EvidenceHeight:  evi.Height(),
					EvidenceTime:    evi.Time(),
					EvidenceAddress: evi.Address(),
				},
			},
		}

		return tp, nil
	case MockRandomEvidence:
		if err := evi.ValidateBasic(); err != nil {
			return nil, err
		}

		tp := &tmproto.Evidence{
			Sum: &tmproto.Evidence_MockRandomEvidence{
				MockRandomEvidence: &tmproto.MockRandomEvidence{
					EvidenceHeight:  evi.Height(),
					EvidenceTime:    evi.Time(),
					EvidenceAddress: evi.Address(),
					RandBytes:       evi.randBytes,
				},
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
	case *tmproto.Evidence_ConflictingHeadersEvidence:
		return ConflictingHeadersEvidenceFromProto(evi.ConflictingHeadersEvidence)
	case *tmproto.Evidence_LunaticValidatorEvidence:
		return LunaticValidatorEvidenceFromProto(evi.LunaticValidatorEvidence)
	case *tmproto.Evidence_PotentialAmnesiaEvidence:
		return PotentialAmnesiaEvidenceFromProto(evi.PotentialAmnesiaEvidence)
	case *tmproto.Evidence_AmnesiaEvidence:
		return AmensiaEvidenceFromProto(evi.AmnesiaEvidence)
	case *tmproto.Evidence_PhantomValidatorEvidence:
		return PhantomValidatorEvidenceFromProto(evi.PhantomValidatorEvidence)
	case *tmproto.Evidence_MockEvidence:
		me := MockEvidence{
			EvidenceHeight:  evi.MockEvidence.GetEvidenceHeight(),
			EvidenceAddress: evi.MockEvidence.GetEvidenceAddress(),
			EvidenceTime:    evi.MockEvidence.GetEvidenceTime(),
		}
		return me, me.ValidateBasic()
	case *tmproto.Evidence_MockRandomEvidence:
		mre := MockRandomEvidence{
			MockEvidence: MockEvidence{
				EvidenceHeight:  evi.MockRandomEvidence.GetEvidenceHeight(),
				EvidenceAddress: evi.MockRandomEvidence.GetEvidenceAddress(),
				EvidenceTime:    evi.MockRandomEvidence.GetEvidenceTime(),
			},
			randBytes: evi.MockRandomEvidence.RandBytes,
		}
		return mre, mre.ValidateBasic()
	default:
		return nil, errors.New("evidence is not recognized")
	}
}

func init() {
	tmjson.RegisterType(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence")
	tmjson.RegisterType(&ConflictingHeadersEvidence{}, "tendermint/ConflictingHeadersEvidence")
	tmjson.RegisterType(&PhantomValidatorEvidence{}, "tendermint/PhantomValidatorEvidence")
	tmjson.RegisterType(&LunaticValidatorEvidence{}, "tendermint/LunaticValidatorEvidence")
	tmjson.RegisterType(&PotentialAmnesiaEvidence{}, "tendermint/PotentialAmnesiaEvidence")
	tmjson.RegisterType(&AmnesiaEvidence{}, "tendermint/AmnesiaEvidence")
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`
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

// Time returns time of the latest vote.
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return maxTime(dve.VoteA.Timestamp, dve.VoteB.Timestamp)
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
	va := dve.VoteA.ToProto()
	vb := dve.VoteB.ToProto()
	// Signatures must be valid
	if !pubKey.VerifyBytes(VoteSignBytes(chainID, va), dve.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifyBytes(VoteSignBytes(chainID, vb), dve.VoteB.Signature) {
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

func (dve DuplicateVoteEvidence) ToProto() tmproto.DuplicateVoteEvidence {
	voteB := dve.VoteB.ToProto()
	voteA := dve.VoteA.ToProto()
	tp := tmproto.DuplicateVoteEvidence{
		VoteA: voteA,
		VoteB: voteB,
	}
	return tp
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

	dve := new(DuplicateVoteEvidence)

	dve.VoteA = vA
	dve.VoteB = vB

	return dve, dve.ValidateBasic()
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

//-------------------------------------------

// ConflictingHeadersEvidence is primarily used by the light client when it
// observes two conflicting headers, both having 1/3+ of the voting power of
// the currently trusted validator set.
type ConflictingHeadersEvidence struct {
	H1 *SignedHeader `json:"h_1"`
	H2 *SignedHeader `json:"h_2"`
}

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
				Vote:                        alternativeHeader.Commit.GetVote(int32(i)),
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
				Vote:               alternativeHeader.Commit.GetVote(int32(i)),
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
						VoteA: ev.H1.Commit.GetVote(int32(i)),
						VoteB: ev.H2.Commit.GetVote(int32(j)),
					})
				} else {
					// if H1.Round != H2.Round we need to run full detection procedure => not
					// immediately slashable.
					firstVote := ev.H1.Commit.GetVote(int32(i))
					secondVote := ev.H2.Commit.GetVote(int32(j))
					var newEv PotentialAmnesiaEvidence
					if firstVote.Timestamp.Before(secondVote.Timestamp) {
						newEv = PotentialAmnesiaEvidence{
							VoteA: firstVote,
							VoteB: secondVote,
						}
					} else {
						newEv = PotentialAmnesiaEvidence{
							VoteA: secondVote,
							VoteB: firstVote,
						}
					}
					// has the validator incorrectly voted for a previous round
					if newEv.VoteA.Round > newEv.VoteB.Round {
						evList = append(evList, MakeAmnesiaEvidence(newEv, EmptyPOLC()))
					} else {
						evList = append(evList, newEv)
					}
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

// Time returns time of the latest header.
func (ev ConflictingHeadersEvidence) Time() time.Time {
	return maxTime(ev.H1.Time, ev.H2.Time)
}

func (ev ConflictingHeadersEvidence) Address() []byte {
	panic("use ConflictingHeadersEvidence#Split to split evidence into individual pieces")
}

func (ev ConflictingHeadersEvidence) Bytes() []byte {
	pbe := ev.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
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
		return fmt.Errorf("alt commit contains too many signatures: %d, expected no more than %d",
			len(alternativeHeader.Commit.Signatures),
			maxNumValidators)
	}

	// Header must be signed by at least 1/3+ of voting power of currently
	// trusted validator set.
	if err := valSet.VerifyCommitTrusting(
		alternativeHeader.ChainID,
		alternativeHeader.Commit,
		tmmath.Fraction{Numerator: 1, Denominator: 3}); err != nil {
		return fmt.Errorf("alt header does not have 1/3+ of voting power of our validator set: %w", err)
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

func (ev ConflictingHeadersEvidence) ToProto() tmproto.ConflictingHeadersEvidence {
	pbh1 := ev.H1.ToProto()
	pbh2 := ev.H2.ToProto()

	tp := tmproto.ConflictingHeadersEvidence{
		H1: pbh1,
		H2: pbh2,
	}
	return tp
}

func ConflictingHeadersEvidenceFromProto(pb *tmproto.ConflictingHeadersEvidence) (ConflictingHeadersEvidence, error) {
	if pb == nil {
		return ConflictingHeadersEvidence{}, errors.New("nil ConflictingHeadersEvidence")
	}
	h1, err := SignedHeaderFromProto(pb.H1)
	if err != nil {
		return ConflictingHeadersEvidence{}, fmt.Errorf("from proto err: %w", err)
	}
	h2, err := SignedHeaderFromProto(pb.H2)
	if err != nil {
		return ConflictingHeadersEvidence{}, fmt.Errorf("from proto err: %w", err)
	}

	tp := ConflictingHeadersEvidence{
		H1: h1,
		H2: h2,
	}

	return tp, tp.ValidateBasic()
}

//-------------------------------------------

type PhantomValidatorEvidence struct {
	Vote                        *Vote `json:"vote"`
	LastHeightValidatorWasInSet int64 `json:"last_height_validator_was_in_set"`
}

var _ Evidence = PhantomValidatorEvidence{}

func (e PhantomValidatorEvidence) Height() int64 {
	return e.Vote.Height
}

// Time returns the Vote's timestamp.
func (e PhantomValidatorEvidence) Time() time.Time {
	return e.Vote.Timestamp
}

func (e PhantomValidatorEvidence) Address() []byte {
	return e.Vote.ValidatorAddress
}

func (e PhantomValidatorEvidence) Hash() []byte {
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}
	return tmhash.Sum(bz)
}

func (e PhantomValidatorEvidence) Bytes() []byte {
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (e PhantomValidatorEvidence) Verify(chainID string, pubKey crypto.PubKey) error {

	v := e.Vote.ToProto()
	if !pubKey.VerifyBytes(VoteSignBytes(chainID, v), e.Vote.Signature) {
		return errors.New("invalid signature")
	}

	return nil
}

func (e PhantomValidatorEvidence) Equal(ev Evidence) bool {
	switch e2 := ev.(type) {
	case PhantomValidatorEvidence:
		return e.Vote.Height == e2.Vote.Height &&
			bytes.Equal(e.Vote.ValidatorAddress, e2.Vote.ValidatorAddress)
	case *PhantomValidatorEvidence:
		return e.Vote.Height == e2.Vote.Height &&
			bytes.Equal(e.Vote.ValidatorAddress, e2.Vote.ValidatorAddress)
	default:
		return false
	}
}

func (e PhantomValidatorEvidence) ValidateBasic() error {

	if e.Vote == nil {
		return errors.New("empty vote")
	}

	if err := e.Vote.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid signature: %v", err)
	}

	if !e.Vote.BlockID.IsComplete() {
		return errors.New("expected vote for block")
	}

	if e.LastHeightValidatorWasInSet <= 0 {
		return errors.New("negative or zero LastHeightValidatorWasInSet")
	}

	return nil
}

func (e PhantomValidatorEvidence) String() string {
	return fmt.Sprintf("PhantomValidatorEvidence{%X voted at height %d}",
		e.Vote.ValidatorAddress, e.Vote.Height)
}

func (e PhantomValidatorEvidence) ToProto() tmproto.PhantomValidatorEvidence {
	vpb := e.Vote.ToProto()

	tp := tmproto.PhantomValidatorEvidence{
		Vote:                        vpb,
		LastHeightValidatorWasInSet: e.LastHeightValidatorWasInSet,
	}

	return tp
}

func PhantomValidatorEvidenceFromProto(pb *tmproto.PhantomValidatorEvidence) (PhantomValidatorEvidence, error) {
	if pb == nil {
		return PhantomValidatorEvidence{}, errors.New("nil PhantomValidatorEvidence")
	}

	vpb, err := VoteFromProto(pb.Vote)
	if err != nil {
		return PhantomValidatorEvidence{}, err
	}

	tp := PhantomValidatorEvidence{
		Vote:                        vpb,
		LastHeightValidatorWasInSet: pb.LastHeightValidatorWasInSet,
	}

	return tp, tp.ValidateBasic()
}

//-------------------------------------------

type LunaticValidatorEvidence struct {
	Header             *Header `json:"header"`
	Vote               *Vote   `json:"vote"`
	InvalidHeaderField string  `json:"invalid_header_field"`
}

var _ Evidence = LunaticValidatorEvidence{}

func (e LunaticValidatorEvidence) Height() int64 {
	return e.Header.Height
}

// Time returns the maximum between the header's time and vote's time.
func (e LunaticValidatorEvidence) Time() time.Time {
	return maxTime(e.Header.Time, e.Vote.Timestamp)
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
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (e LunaticValidatorEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// chainID must be the same
	if chainID != e.Header.ChainID {
		return fmt.Errorf("chainID do not match: %s vs %s",
			chainID,
			e.Header.ChainID,
		)
	}

	v := e.Vote.ToProto()
	if !pubKey.VerifyBytes(VoteSignBytes(chainID, v), e.Vote.Signature) {
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

	if committedHeader == nil {
		return errors.New("committed header is nil")
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

func (e LunaticValidatorEvidence) ToProto() tmproto.LunaticValidatorEvidence {
	h := e.Header.ToProto()
	v := e.Vote.ToProto()

	tp := tmproto.LunaticValidatorEvidence{
		Header:             h,
		Vote:               v,
		InvalidHeaderField: e.InvalidHeaderField,
	}

	return tp
}

func LunaticValidatorEvidenceFromProto(pb *tmproto.LunaticValidatorEvidence) (*LunaticValidatorEvidence, error) {
	if pb == nil {
		return nil, errors.New("nil LunaticValidatorEvidence")
	}

	h, err := HeaderFromProto(pb.GetHeader())
	if err != nil {
		return nil, err
	}

	v, err := VoteFromProto(pb.GetVote())
	if err != nil {
		return nil, err
	}

	tp := LunaticValidatorEvidence{
		Header:             &h,
		Vote:               v,
		InvalidHeaderField: pb.InvalidHeaderField,
	}

	return &tp, tp.ValidateBasic()
}

//-------------------------------------------

// PotentialAmnesiaEvidence is constructed when a validator votes on two different blocks at different rounds
// in the same height. PotentialAmnesiaEvidence can then evolve into AmensiaEvidence if the indicted validator
// is incapable of providing the proof of lock change that validates voting twice in the allotted trial period.
// Heightstamp is used for each node to keep a track of how much time has passed so as to know when the trial period
// is finished and is set when the node first receives the evidence.
type PotentialAmnesiaEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`

	HeightStamp int64
}

var _ Evidence = PotentialAmnesiaEvidence{}

func (e PotentialAmnesiaEvidence) Height() int64 {
	return e.VoteA.Height
}

// Time returns time of the latest vote.
func (e PotentialAmnesiaEvidence) Time() time.Time {
	return maxTime(e.VoteA.Timestamp, e.VoteB.Timestamp)
}

func (e PotentialAmnesiaEvidence) Address() []byte {
	return e.VoteA.ValidatorAddress
}

func (e PotentialAmnesiaEvidence) Hash() []byte {
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return tmhash.Sum(bz)
}

func (e PotentialAmnesiaEvidence) Bytes() []byte {
	pbe := e.ToProto()

	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (e PotentialAmnesiaEvidence) Verify(chainID string, pubKey crypto.PubKey) error {
	// pubkey must match address (this should already be true, sanity check)
	addr := e.VoteA.ValidatorAddress
	if !bytes.Equal(pubKey.Address(), addr) {
		return fmt.Errorf("address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, pubKey.Address())
	}

	va := e.VoteA.ToProto()
	vb := e.VoteB.ToProto()

	// Signatures must be valid
	if !pubKey.VerifyBytes(VoteSignBytes(chainID, va), e.VoteA.Signature) {
		return fmt.Errorf("verifying VoteA: %w", ErrVoteInvalidSignature)
	}
	if !pubKey.VerifyBytes(VoteSignBytes(chainID, vb), e.VoteB.Signature) {
		return fmt.Errorf("verifying VoteB: %w", ErrVoteInvalidSignature)
	}

	return nil
}

func (e PotentialAmnesiaEvidence) Equal(ev Evidence) bool {
	switch e2 := ev.(type) {
	case PotentialAmnesiaEvidence:
		return e.Height() == e2.Height() && e.VoteA.Round == e2.VoteA.Round && e.VoteB.Round == e2.VoteB.Round &&
			bytes.Equal(e.Address(), e2.Address())
	case *PotentialAmnesiaEvidence:
		return e.Height() == e2.Height() && e.VoteA.Round == e2.VoteA.Round && e.VoteB.Round == e2.VoteB.Round &&
			bytes.Equal(e.Address(), e2.Address())
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

	// H/S must be the same
	if e.VoteA.Height != e.VoteB.Height ||
		e.VoteA.Type != e.VoteB.Type {
		return fmt.Errorf("h/s do not match: %d/%v vs %d/%v",
			e.VoteA.Height, e.VoteA.Type, e.VoteB.Height, e.VoteB.Type)
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

// Primed finds whether the PotentialAmnesiaEvidence is ready to be upgraded to Amnesia Evidence. It is decided if
// either the prosecuted node voted in the past or if the allocated trial period has expired without a proof of lock
// change having been provided.
func (e PotentialAmnesiaEvidence) Primed(trialPeriod, currentHeight int64) bool {
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

func (e PotentialAmnesiaEvidence) ToProto() tmproto.PotentialAmnesiaEvidence {
	voteB := e.VoteB.ToProto()
	voteA := e.VoteA.ToProto()

	tp := tmproto.PotentialAmnesiaEvidence{
		VoteA:       voteA,
		VoteB:       voteB,
		HeightStamp: e.HeightStamp,
	}

	return tp
}

// ------------------

// ProofOfLockChange (POLC) proves that a node followed the consensus protocol and voted for a precommit in two
// different rounds because the node received a majority of votes for a different block in the latter round. In cases of
// amnesia evidence, a suspected node will need ProofOfLockChange to prove that the node did not break protocol.
type ProofOfLockChange struct {
	Votes  []Vote        `json:"votes"`
	PubKey crypto.PubKey `json:"pubkey"`
}

// MakePOLCFromVoteSet can be used when a majority of prevotes or precommits for a block is seen
// that the node has itself not yet voted for in order to process the vote set into a proof of lock change
func MakePOLCFromVoteSet(voteSet *VoteSet, pubKey crypto.PubKey, blockID BlockID) (ProofOfLockChange, error) {
	polc := makePOLCFromVoteSet(voteSet, pubKey, blockID)
	return polc, polc.ValidateBasic()
}

func makePOLCFromVoteSet(voteSet *VoteSet, pubKey crypto.PubKey, blockID BlockID) ProofOfLockChange {
	var votes []Vote
	valSetSize := voteSet.Size()
	for valIdx := int32(0); int(valIdx) < valSetSize; valIdx++ {
		vote := voteSet.GetByIndex(valIdx)
		if vote != nil && vote.BlockID.Equals(blockID) {
			votes = append(votes, *vote)
		}
	}
	return ProofOfLockChange{
		Votes:  votes,
		PubKey: pubKey,
	}
}

// EmptyPOLC returns an empty polc. This is used when no polc has been provided in the allocated trial period time
// and the node now needs to move to upgrading to AmnesiaEvidence and hence uses an empty polc
func EmptyPOLC() ProofOfLockChange {
	return ProofOfLockChange{
		nil,
		nil,
	}
}

func (e ProofOfLockChange) Height() int64 {
	return e.Votes[0].Height
}

// Time returns time of the latest vote.
func (e ProofOfLockChange) Time() time.Time {
	latest := e.Votes[0].Timestamp
	for _, vote := range e.Votes {
		if vote.Timestamp.After(latest) {
			latest = vote.Timestamp
		}
	}
	return latest
}

func (e ProofOfLockChange) Round() int32 {
	return e.Votes[0].Round
}

func (e ProofOfLockChange) Address() []byte {
	return e.PubKey.Address()
}

func (e ProofOfLockChange) BlockID() BlockID {
	return e.Votes[0].BlockID
}

// ValidateVotes checks the polc against the validator set of that height. The function makes sure that the polc
// contains a majority of votes and that each
func (e ProofOfLockChange) ValidateVotes(valSet *ValidatorSet, chainID string) error {
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
				if !validator.PubKey.VerifyBytes(VoteSignBytes(chainID, v), vote.Signature) {
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

func (e ProofOfLockChange) Equal(e2 ProofOfLockChange) bool {
	return bytes.Equal(e.Address(), e2.Address()) && e.Height() == e2.Height() &&
		e.Round() == e2.Round()
}

func (e ProofOfLockChange) ValidateBasic() error {
	// first check if the polc is absent / empty
	if e.IsAbsent() {
		return nil
	}

	if e.PubKey == nil {
		return errors.New("missing public key")
	}
	// validate basic doesn't count the number of votes and their voting power, this is to be done by VerifyEvidence
	if e.Votes == nil {
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

func (e ProofOfLockChange) String() string {
	if e.IsAbsent() {
		return "Empty ProofOfLockChange"
	}
	return fmt.Sprintf("ProofOfLockChange {Address: %X, Height: %d, Round: %d", e.Address(), e.Height(),
		e.Votes[0].Round)
}

// IsAbsent checks if the polc is empty
func (e ProofOfLockChange) IsAbsent() bool {
	return e.Votes == nil && e.PubKey == nil
}

func (e *ProofOfLockChange) ToProto() (*tmproto.ProofOfLockChange, error) {
	if e == nil {
		return nil, errors.New("nil proof of lock change")
	}
	plc := new(tmproto.ProofOfLockChange)
	vpb := make([]*tmproto.Vote, len(e.Votes))

	// if absent create empty proto polc
	if e.IsAbsent() {
		return plc, nil
	}

	if e.Votes == nil {
		return nil, errors.New("polc is not absent but has no votes")
	}
	for i, v := range e.Votes {
		pb := v.ToProto()
		if pb != nil {
			vpb[i] = pb
		}
	}

	pk, err := cryptoenc.PubKeyToProto(e.PubKey)
	if err != nil {
		return nil, err
	}
	plc.PubKey = &pk
	plc.Votes = vpb

	return plc, nil
}

// AmnesiaEvidence is the progression of PotentialAmnesiaEvidence and is used to prove an infringement of the
// Tendermint consensus when a validator incorrectly sends a vote in a later round without correctly changing the lock
type AmnesiaEvidence struct {
	PotentialAmnesiaEvidence
	Polc ProofOfLockChange
}

// Height, Time, Address and Verify functions are all inherited by the PotentialAmnesiaEvidence struct
var _ Evidence = &AmnesiaEvidence{}
var _ Evidence = AmnesiaEvidence{}

func MakeAmnesiaEvidence(pe PotentialAmnesiaEvidence, proof ProofOfLockChange) AmnesiaEvidence {
	return AmnesiaEvidence{
		pe,
		proof,
	}
}

func (e AmnesiaEvidence) Equal(ev Evidence) bool {
	e2, ok := ev.(AmnesiaEvidence)
	if !ok {
		return false
	}
	return e.PotentialAmnesiaEvidence.Equal(e2.PotentialAmnesiaEvidence)
}

func (e AmnesiaEvidence) ValidateBasic() error {
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

// ViolatedConsensus assess on the basis of the AmensiaEvidence whether the validator has violated the
// Tendermint consensus. Evidence must be validated first (see ValidateBasic).
// We are only interested in proving that the latter of the votes in terms of time was correctly done.
func (e AmnesiaEvidence) ViolatedConsensus() (bool, string) {
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

func (e AmnesiaEvidence) String() string {
	return fmt.Sprintf("AmnesiaEvidence{ %v, polc: %v }", e.PotentialAmnesiaEvidence, e.Polc)
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

	vpb := make([]Vote, len(pb.Votes))
	for i, v := range pb.Votes {
		vi, err := VoteFromProto(v)
		if err != nil {
			return nil, err
		}
		vpb[i] = *vi
	}

	if pb.PubKey == nil {
		return nil, errors.New("proofOfLockChange: is not abest but has nil PubKey")
	}
	pk, err := cryptoenc.PubKeyFromProto(*pb.PubKey)
	if err != nil {
		return nil, err
	}

	plc.PubKey = pk
	plc.Votes = vpb

	return plc, nil
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
	}

	return &tp, tp.ValidateBasic()
}

func AmnesiaEvidenceToProto(evi AmnesiaEvidence) (*tmproto.Evidence, error) {
	paepb := evi.PotentialAmnesiaEvidence.ToProto()

	polc, err := evi.Polc.ToProto()
	if err != nil {
		return nil, err
	}

	tp := &tmproto.Evidence{
		Sum: &tmproto.Evidence_AmnesiaEvidence{
			AmnesiaEvidence: &tmproto.AmnesiaEvidence{
				PotentialAmnesiaEvidence: &paepb,
				Polc:                     polc,
			},
		},
	}

	return tp, nil
}

func AmensiaEvidenceFromProto(pb *tmproto.AmnesiaEvidence) (AmnesiaEvidence, error) {
	if pb == nil {
		return AmnesiaEvidence{}, errors.New("nil duplicate vote evidence")
	}

	pae, err := PotentialAmnesiaEvidenceFromProto(pb.PotentialAmnesiaEvidence)
	if err != nil {
		return AmnesiaEvidence{}, err
	}
	polc, err := ProofOfLockChangeFromProto(pb.Polc)
	if err != nil {
		return AmnesiaEvidence{}, err
	}

	tp := AmnesiaEvidence{
		PotentialAmnesiaEvidence: *pae,
		Polc:                     *polc,
	}

	return tp, tp.ValidateBasic()
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

func (e MockRandomEvidence) Equal(ev Evidence) bool { return false }

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
		EvidenceAddress: address,
	}
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
	return e.EvidenceHeight == ev.Height() &&
		bytes.Equal(e.EvidenceAddress, ev.Address())
}
func (e MockEvidence) ValidateBasic() error { return nil }
func (e MockEvidence) String() string {
	return fmt.Sprintf("Evidence: %d/%s/%s", e.EvidenceHeight, e.Time(), e.EvidenceAddress)
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
		Votes:  []Vote{vote},
		PubKey: pubKey,
	}
}

func maxTime(t1 time.Time, t2 time.Time) time.Time {
	if t1.After(t2) {
		return t1
	}
	return t2
}
