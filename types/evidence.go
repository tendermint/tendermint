package types

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/internal/jsontypes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Evidence represents any provable malicious activity by a validator.
// Verification logic for each evidence is part of the evidence module.
type Evidence interface {
	ABCI() []abci.Misbehavior // forms individual evidence to be sent to the application
	Bytes() []byte            // bytes which comprise the evidence
	Hash() []byte             // hash of the evidence
	Height() int64            // height of the infraction
	String() string           // string format of the evidence
	Time() time.Time          // time of the infraction
	ValidateBasic() error     // basic consistency check

	// Implementations must support tagged encoding in JSON.
	jsontypes.Tagged
}

//--------------------------------------------------------------------------------------

// DuplicateVoteEvidence contains evidence of a single validator signing two conflicting votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`

	// abci specific information
	TotalVotingPower int64 `json:",string"`
	ValidatorPower   int64 `json:",string"`
	Timestamp        time.Time
}

// TypeTag implements the jsontypes.Tagged interface.
func (*DuplicateVoteEvidence) TypeTag() string { return "tendermint/DuplicateVoteEvidence" }

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If either of the votes is nil, the val set is nil or the voter is
// not in the val set, an error is returned
func NewDuplicateVoteEvidence(vote1, vote2 *Vote, blockTime time.Time, valSet *ValidatorSet,
) (*DuplicateVoteEvidence, error) {
	var voteA, voteB *Vote
	if vote1 == nil || vote2 == nil {
		return nil, errors.New("missing vote")
	}
	if valSet == nil {
		return nil, errors.New("missing validator set")
	}
	idx, val := valSet.GetByAddress(vote1.ValidatorAddress)
	if idx == -1 {
		return nil, errors.New("validator not in validator set")
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
	}, nil
}

// ABCI returns the application relevant representation of the evidence
func (dve *DuplicateVoteEvidence) ABCI() []abci.Misbehavior {
	return []abci.Misbehavior{{
		Type: abci.MisbehaviorType_DUPLICATE_VOTE,
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
		panic("marshaling duplicate vote evidence to bytes: " + err.Error())
	}

	return bz
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() []byte {
	return crypto.Checksum(dve.Bytes())
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

// ValidateABCI validates the ABCI component of the evidence by checking the
// timestamp, validator power and total voting power.
func (dve *DuplicateVoteEvidence) ValidateABCI(
	val *Validator,
	valSet *ValidatorSet,
	evidenceTime time.Time,
) error {

	if dve.Timestamp != evidenceTime {
		return fmt.Errorf(
			"evidence has a different time to the block it is associated with (%v != %v)",
			dve.Timestamp, evidenceTime)
	}

	if val.VotingPower != dve.ValidatorPower {
		return fmt.Errorf("validator power from evidence and our validator set does not match (%d != %d)",
			dve.ValidatorPower, val.VotingPower)
	}
	if valSet.TotalVotingPower() != dve.TotalVotingPower {
		return fmt.Errorf("total voting power from the evidence and our validator set does not match (%d != %d)",
			dve.TotalVotingPower, valSet.TotalVotingPower())
	}

	return nil
}

// GenerateABCI populates the ABCI component of the evidence. This includes the
// validator power, timestamp and total voting power.
func (dve *DuplicateVoteEvidence) GenerateABCI(
	val *Validator,
	valSet *ValidatorSet,
	evidenceTime time.Time,
) {
	dve.ValidatorPower = val.VotingPower
	dve.TotalVotingPower = valSet.TotalVotingPower()
	dve.Timestamp = evidenceTime
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

	var vA *Vote
	if pb.VoteA != nil {
		var err error
		vA, err = VoteFromProto(pb.VoteA)
		if err != nil {
			return nil, err
		}
		if err = vA.ValidateBasic(); err != nil {
			return nil, err
		}
	}

	var vB *Vote
	if pb.VoteB != nil {
		var err error
		vB, err = VoteFromProto(pb.VoteB)
		if err != nil {
			return nil, err
		}
		if err = vB.ValidateBasic(); err != nil {
			return nil, err
		}
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
//
// CommonHeight is used to indicate the type of attack. If the height is different to the conflicting block
// height, then nodes will treat this as of the Lunatic form, else it is of the Equivocation form.
type LightClientAttackEvidence struct {
	ConflictingBlock *LightBlock
	CommonHeight     int64 `json:",string"`

	// abci specific information
	ByzantineValidators []*Validator // validators in the validator set that misbehaved in creating the conflicting block
	TotalVotingPower    int64        `json:",string"` // total voting power of the validator set at the common height
	Timestamp           time.Time    // timestamp of the block at the common height
}

// TypeTag implements the jsontypes.Tagged interface.
func (*LightClientAttackEvidence) TypeTag() string { return "tendermint/LightClientAttackEvidence" }

var _ Evidence = &LightClientAttackEvidence{}

// ABCI forms an array of abci.Misbehavior for each byzantine validator
func (l *LightClientAttackEvidence) ABCI() []abci.Misbehavior {
	abciEv := make([]abci.Misbehavior, len(l.ByzantineValidators))
	for idx, val := range l.ByzantineValidators {
		abciEv[idx] = abci.Misbehavior{
			Type:             abci.MisbehaviorType_LIGHT_CLIENT_ATTACK,
			Validator:        TM2PB.Validator(val),
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
		panic("converting light client attack evidence to proto: " + err.Error())
	}
	bz, err := pbe.Marshal()
	if err != nil {
		panic("marshaling light client attack evidence to bytes: " + err.Error())
	}
	return bz
}

// GetByzantineValidators finds out what style of attack LightClientAttackEvidence was and then works out who
// the malicious validators were and returns them. This is used both for forming the ByzantineValidators
// field and for validating that it is correct. Validators are ordered based on validator power
func (l *LightClientAttackEvidence) GetByzantineValidators(commonVals *ValidatorSet,
	trusted *SignedHeader) []*Validator {
	var validators []*Validator
	// First check if the header is invalid. This means that it is a lunatic attack and therefore we take the
	// validators who are in the commonVals and voted for the lunatic header
	if l.ConflictingHeaderIsInvalid(trusted.Header) {
		for _, commitSig := range l.ConflictingBlock.Commit.Signatures {
			if commitSig.BlockIDFlag != BlockIDFlagCommit {
				continue
			}

			_, val := commonVals.GetByAddress(commitSig.ValidatorAddress)
			if val == nil {
				// validator wasn't in the common validator set
				continue
			}
			validators = append(validators, val)
		}
		sort.Sort(ValidatorsByVotingPower(validators))
		return validators
	} else if trusted.Commit.Round == l.ConflictingBlock.Commit.Round {
		// This is an equivocation attack as both commits are in the same round. We then find the validators
		// from the conflicting light block validator set that voted in both headers.
		// Validator hashes are the same therefore the indexing order of validators are the same and thus we
		// only need a single loop to find the validators that voted twice.
		for i := 0; i < len(l.ConflictingBlock.Commit.Signatures); i++ {
			sigA := l.ConflictingBlock.Commit.Signatures[i]
			if sigA.BlockIDFlag != BlockIDFlagCommit {
				continue
			}

			sigB := trusted.Commit.Signatures[i]
			if sigB.BlockIDFlag != BlockIDFlagCommit {
				continue
			}

			_, val := l.ConflictingBlock.ValidatorSet.GetByAddress(sigA.ValidatorAddress)
			validators = append(validators, val)
		}
		sort.Sort(ValidatorsByVotingPower(validators))
		return validators
	}
	// if the rounds are different then this is an amnesia attack. Unfortunately, given the nature of the attack,
	// we aren't able yet to deduce which are malicious validators and which are not hence we return an
	// empty validator set.
	return validators
}

// ConflictingHeaderIsInvalid takes a trusted header and matches it againt a conflicting header
// to determine whether the conflicting header was the product of a valid state transition
// or not. If it is then all the deterministic fields of the header should be the same.
// If not, it is an invalid header and constitutes a lunatic attack.
func (l *LightClientAttackEvidence) ConflictingHeaderIsInvalid(trustedHeader *Header) bool {
	return !bytes.Equal(trustedHeader.ValidatorsHash, l.ConflictingBlock.ValidatorsHash) ||
		!bytes.Equal(trustedHeader.NextValidatorsHash, l.ConflictingBlock.NextValidatorsHash) ||
		!bytes.Equal(trustedHeader.ConsensusHash, l.ConflictingBlock.ConsensusHash) ||
		!bytes.Equal(trustedHeader.AppHash, l.ConflictingBlock.AppHash) ||
		!bytes.Equal(trustedHeader.LastResultsHash, l.ConflictingBlock.LastResultsHash)

}

// Hash returns the hash of the header and the commonHeight. This is designed to cause hash collisions
// with evidence that have the same conflicting header and common height but different permutations
// of validator commit signatures. The reason for this is that we don't want to allow several
// permutations of the same evidence to be committed on chain. Ideally we commit the header with the
// most commit signatures (captures the most byzantine validators) but anything greater than 1/3 is
// sufficient.
// TODO: We should change the hash to include the commit, header, total voting power, byzantine
// validators and timestamp
func (l *LightClientAttackEvidence) Hash() []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, l.CommonHeight)
	bz := make([]byte, crypto.HashSize+n)
	copy(bz[:crypto.HashSize-1], l.ConflictingBlock.Hash().Bytes())
	copy(bz[crypto.HashSize:], buf)
	return crypto.Checksum(bz)
}

// Height returns the last height at which the primary provider and witness provider had the same header.
// We use this as the height of the infraction rather than the actual conflicting header because we know
// that the malicious validators were bonded at this height which is important for evidence expiry
func (l *LightClientAttackEvidence) Height() int64 {
	return l.CommonHeight
}

// String returns a string representation of LightClientAttackEvidence
func (l *LightClientAttackEvidence) String() string {
	return fmt.Sprintf(`LightClientAttackEvidence{
		ConflictingBlock: %v,
		CommonHeight: %d,
		ByzatineValidators: %v,
		TotalVotingPower: %d,
		Timestamp: %v}#%X`,
		l.ConflictingBlock.String(), l.CommonHeight, l.ByzantineValidators,
		l.TotalVotingPower, l.Timestamp, l.Hash())
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

	if l.TotalVotingPower <= 0 {
		return errors.New("negative or zero total voting power")
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

	if err := l.ConflictingBlock.ValidateBasic(l.ConflictingBlock.ChainID); err != nil {
		return fmt.Errorf("invalid conflicting light block: %w", err)
	}

	return nil
}

// ValidateABCI validates the ABCI component of the evidence by checking the
// timestamp, byzantine validators and total voting power all match. ABCI
// components are validated separately because they can be re generated if
// invalid.
func (l *LightClientAttackEvidence) ValidateABCI(
	commonVals *ValidatorSet,
	trustedHeader *SignedHeader,
	evidenceTime time.Time,
) error {

	if evTotal, valsTotal := l.TotalVotingPower, commonVals.TotalVotingPower(); evTotal != valsTotal {
		return fmt.Errorf("total voting power from the evidence and our validator set does not match (%d != %d)",
			evTotal, valsTotal)
	}

	if l.Timestamp != evidenceTime {
		return fmt.Errorf(
			"evidence has a different time to the block it is associated with (%v != %v)",
			l.Timestamp, evidenceTime)
	}

	// Find out what type of attack this was and thus extract the malicious
	// validators. Note, in the case of an Amnesia attack we don't have any
	// malicious validators.
	validators := l.GetByzantineValidators(commonVals, trustedHeader)

	// Ensure this matches the validators that are listed in the evidence. They
	// should be ordered based on power.
	if validators == nil && l.ByzantineValidators != nil {
		return fmt.Errorf(
			"expected nil validators from an amnesia light client attack but got %d",
			len(l.ByzantineValidators),
		)
	}

	if exp, got := len(validators), len(l.ByzantineValidators); exp != got {
		return fmt.Errorf("expected %d byzantine validators from evidence but got %d", exp, got)
	}

	for idx, val := range validators {
		if !bytes.Equal(l.ByzantineValidators[idx].Address, val.Address) {
			return fmt.Errorf(
				"evidence contained an unexpected byzantine validator address; expected: %v, got: %v",
				val.Address, l.ByzantineValidators[idx].Address,
			)
		}

		if l.ByzantineValidators[idx].VotingPower != val.VotingPower {
			return fmt.Errorf(
				"evidence contained unexpected byzantine validator power; expected %d, got %d",
				val.VotingPower, l.ByzantineValidators[idx].VotingPower,
			)
		}
	}

	return nil
}

// GenerateABCI populates the ABCI component of the evidence: the timestamp,
// total voting power and byantine validators
func (l *LightClientAttackEvidence) GenerateABCI(
	commonVals *ValidatorSet,
	trustedHeader *SignedHeader,
	evidenceTime time.Time,
) {
	l.Timestamp = evidenceTime
	l.TotalVotingPower = commonVals.TotalVotingPower()
	l.ByzantineValidators = l.GetByzantineValidators(commonVals, trustedHeader)
}

// ToProto encodes LightClientAttackEvidence to protobuf
func (l *LightClientAttackEvidence) ToProto() (*tmproto.LightClientAttackEvidence, error) {
	conflictingBlock, err := l.ConflictingBlock.ToProto()
	if err != nil {
		return nil, err
	}

	byzVals := make([]*tmproto.Validator, len(l.ByzantineValidators))
	for idx, val := range l.ByzantineValidators {
		valpb, err := val.ToProto()
		if err != nil {
			return nil, err
		}
		byzVals[idx] = valpb
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

	byzVals := make([]*Validator, len(lpb.ByzantineValidators))
	for idx, valpb := range lpb.ByzantineValidators {
		val, err := ValidatorFromProto(valpb)
		if err != nil {
			return nil, err
		}
		byzVals[idx] = val
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

// StringIndented returns a string representation of the evidence.
func (evl EvidenceList) StringIndented(indent string) string {
	if evl == nil {
		return "nil-Evidence"
	}
	evStrings := make([]string, tmmath.MinInt(len(evl), 21))
	for i, ev := range evl {
		if i == 20 {
			evStrings[i] = fmt.Sprintf("... (%v total)", len(evl))
			break
		}
		evStrings[i] = fmt.Sprintf("Evidence:%v", ev)
	}
	return fmt.Sprintf(`EvidenceList{
%s  %v
%s}#%v`,
		indent, strings.Join(evStrings, "\n"+indent+"  "),
		indent, evl.Hash())
}

// ByteSize returns the total byte size of all the evidence
func (evl EvidenceList) ByteSize() int64 {
	if len(evl) != 0 {
		pb, err := evl.ToProto()
		if err != nil {
			panic(err)
		}
		return int64(pb.Size())
	}
	return 0
}

// FromProto sets a protobuf EvidenceList to the given pointer.
func (evl *EvidenceList) FromProto(eviList *tmproto.EvidenceList) error {
	if eviList == nil {
		return errors.New("nil evidence list")
	}

	eviBzs := make(EvidenceList, len(eviList.Evidence))
	for i := range eviList.Evidence {
		evi, err := EvidenceFromProto(&eviList.Evidence[i])
		if err != nil {
			return err
		}
		eviBzs[i] = evi
	}
	*evl = eviBzs
	return nil
}

// ToProto converts EvidenceList to protobuf
func (evl *EvidenceList) ToProto() (*tmproto.EvidenceList, error) {
	if evl == nil {
		return nil, errors.New("nil evidence list")
	}

	eviBzs := make([]tmproto.Evidence, len(*evl))
	for i, v := range *evl {
		protoEvi, err := EvidenceToProto(v)
		if err != nil {
			return nil, err
		}
		eviBzs[i] = *protoEvi
	}
	return &tmproto.EvidenceList{Evidence: eviBzs}, nil
}

func (evl EvidenceList) MarshalJSON() ([]byte, error) {
	lst := make([]json.RawMessage, len(evl))
	for i, ev := range evl {
		bits, err := jsontypes.Marshal(ev)
		if err != nil {
			return nil, err
		}
		lst[i] = bits
	}
	return json.Marshal(lst)
}

func (evl *EvidenceList) UnmarshalJSON(data []byte) error {
	var lst []json.RawMessage
	if err := json.Unmarshal(data, &lst); err != nil {
		return err
	}
	out := make([]Evidence, len(lst))
	for i, elt := range lst {
		if err := jsontypes.Unmarshal(elt, &out[i]); err != nil {
			return err
		}
	}
	*evl = EvidenceList(out)
	return nil
}

// Hash returns the simple merkle root hash of the EvidenceList.
func (evl EvidenceList) Hash() []byte {
	// These allocations are required because Evidence is not of type Bytes, and
	// golang slices can't be typed cast. This shouldn't be a performance problem since
	// the Evidence size is capped.
	evidenceBzs := make([][]byte, len(evl))
	for i := 0; i < len(evl); i++ {
		// TODO: We should change this to the hash. Using bytes contains some unexported data that
		// may cause different hashes
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

// ToABCI converts the evidence list to a slice of the ABCI protobuf messages
// for use when communicating the evidence to an application.
func (evl EvidenceList) ToABCI() []abci.Misbehavior {
	var el []abci.Misbehavior
	for _, e := range evl {
		el = append(el, e.ABCI()...)
	}
	return el
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
	jsontypes.MustRegister((*DuplicateVoteEvidence)(nil))
	jsontypes.MustRegister((*LightClientAttackEvidence)(nil))
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
func NewMockDuplicateVoteEvidence(ctx context.Context, height int64, time time.Time, chainID string) (*DuplicateVoteEvidence, error) {
	val := NewMockPV()
	return NewMockDuplicateVoteEvidenceWithValidator(ctx, height, time, val, chainID)
}

// assumes voting power to be 10 and validator to be the only one in the set
func NewMockDuplicateVoteEvidenceWithValidator(ctx context.Context, height int64, time time.Time, pv PrivValidator, chainID string) (*DuplicateVoteEvidence, error) {
	pubKey, err := pv.GetPubKey(ctx)
	if err != nil {
		return nil, err
	}

	val := NewValidator(pubKey, 10)
	voteA := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vA := voteA.ToProto()
	_ = pv.SignVote(ctx, chainID, vA)
	voteA.Signature = vA.Signature
	voteB := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vB := voteB.ToProto()
	_ = pv.SignVote(ctx, chainID, vB)
	voteB.Signature = vB.Signature
	ev, err := NewDuplicateVoteEvidence(voteA, voteB, time, NewValidatorSet([]*Validator{val}))
	if err != nil {
		return nil, fmt.Errorf("constructing mock duplicate vote evidence: %w", err)
	}
	return ev, nil
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
		Hash: tmrand.Bytes(crypto.HashSize),
		PartSetHeader: PartSetHeader{
			Total: 1,
			Hash:  tmrand.Bytes(crypto.HashSize),
		},
	}
}
