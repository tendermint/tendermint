package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/config"
)

const (
	VoteTypeBare      = byte(0x00)
	VoteTypePrecommit = byte(0x01)
	VoteTypeCommit    = byte(0x02)
)

var (
	ErrVoteUnexpectedPhase      = errors.New("Unexpected phase")
	ErrVoteInvalidAccount       = errors.New("Invalid round vote account")
	ErrVoteInvalidSignature     = errors.New("Invalid round vote signature")
	ErrVoteInvalidHash          = errors.New("Invalid hash")
	ErrVoteConflictingSignature = errors.New("Conflicting round vote signature")
)

// Represents a bare, precommit, or commit vote for proposals.
type Vote struct {
	Height uint32
	Round  uint16
	Type   byte
	Hash   []byte // empty if vote is nil.
	Signature
}

func ReadVote(r io.Reader) *Vote {
	return &Vote{
		Height:    uint32(ReadUInt32(r)),
		Round:     uint16(ReadUInt16(r)),
		Type:      byte(ReadByte(r)),
		Hash:      ReadByteSlice(r),
		Signature: ReadSignature(r),
	}
}

func (v *Vote) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(UInt32(v.Height), w, n, err)
	n, err = WriteTo(UInt16(v.Round), w, n, err)
	n, err = WriteTo(Byte(v.Type), w, n, err)
	n, err = WriteTo(ByteSlice(v.Hash), w, n, err)
	n, err = WriteTo(v.Signature, w, n, err)
	return
}

// This is the byteslice that validators should sign to signify a vote
// for the given proposal at given height & round.
// If hash is nil, the vote is a nil vote.
func (v *Vote) GetDocument() []byte {
	switch v.Type {
	case VoteTypeBare:
		if len(v.Hash) == 0 {
			doc := fmt.Sprintf("%v://consensus/%v/%v/b\nnil",
				config.Config.Network, v.Height, v.Round)
			return []byte(doc)
		} else {
			doc := fmt.Sprintf("%v://consensus/%v/%v/b\n%v",
				config.Config.Network, v.Height, v.Round,
				CalcBlockURI(v.Height, v.Hash))
			return []byte(doc)
		}
	case VoteTypePrecommit:
		if len(v.Hash) == 0 {
			doc := fmt.Sprintf("%v://consensus/%v/%v/p\nnil",
				config.Config.Network, v.Height, v.Round)
			return []byte(doc)
		} else {
			doc := fmt.Sprintf("%v://consensus/%v/%v/p\n%v",
				config.Config.Network, v.Height, v.Round,
				CalcBlockURI(v.Height, v.Hash))
			return []byte(doc)
		}
	case VoteTypeCommit:
		if len(v.Hash) == 0 {
			panic("Commit hash cannot be nil")
		} else {
			doc := fmt.Sprintf("%v://consensus/%v/c\n%v",
				config.Config.Network, v.Height, // omit round info
				CalcBlockURI(v.Height, v.Hash))
			return []byte(doc)
		}
	default:
		panic("Unknown vote type")
	}
}

//-----------------------------------------------------------------------------

// VoteSet helps collect signatures from validators at each height+round
// for a predefined vote type.
type VoteSet struct {
	mtx              sync.Mutex
	height           uint32
	round            uint16
	type_            byte
	validators       map[uint64]*Validator
	votes            map[uint64]*Vote
	votesByHash      map[string]uint64
	totalVotes       uint64
	totalVotingPower uint64
}

// Constructs a new VoteSet struct used to accumulate votes for each round.
func NewVoteSet(height uint32, round uint16, type_ byte, validators map[uint64]*Validator) *VoteSet {
	totalVotingPower := uint64(0)
	for _, val := range validators {
		totalVotingPower += uint64(val.VotingPower)
	}
	return &VoteSet{
		height:           height,
		round:            round,
		type_:            type_,
		validators:       validators,
		votes:            make(map[uint64]*Vote, len(validators)),
		votesByHash:      make(map[string]uint64),
		totalVotes:       0,
		totalVotingPower: totalVotingPower,
	}
}

// True if added, false if not.
// Returns ErrVote[UnexpectedPhase|InvalidAccount|InvalidSignature|InvalidHash|ConflictingSignature]
func (vs *VoteSet) AddVote(vote *Vote) (bool, error) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()

	// Make sure the phase matches.
	if vote.Height != vs.height || vote.Round != vs.round || vote.Type != vs.type_ {
		return false, ErrVoteUnexpectedPhase
	}

	val := vs.validators[uint64(vote.SignerId)]
	// Ensure that signer is a validator.
	if val == nil {
		return false, ErrVoteInvalidAccount
	}
	// Check signature.
	if !val.Verify(vote.GetDocument(), vote.Signature.Bytes) {
		// Bad signature.
		return false, ErrVoteInvalidSignature
	}
	// If vote already exists, return false.
	if existingVote, ok := vs.votes[uint64(vote.SignerId)]; ok {
		if bytes.Equal(existingVote.Hash, vote.Hash) {
			return false, nil
		} else {
			return false, ErrVoteConflictingSignature
		}
	}
	vs.votes[uint64(vote.SignerId)] = vote
	vs.votesByHash[string(vote.Hash)] += uint64(val.VotingPower)
	vs.totalVotes += uint64(val.VotingPower)
	return true, nil
}

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, false).
func (vs *VoteSet) TwoThirdsMajority() (hash []byte, ok bool) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	twoThirdsMajority := (vs.totalVotingPower*uint64(2) + uint64(2)) / uint64(3)
	if vs.totalVotes < twoThirdsMajority {
		return nil, false
	}
	for hash, votes := range vs.votesByHash {
		if votes >= twoThirdsMajority {
			if hash == "" {
				return nil, true
			} else {
				return []byte(hash), true
			}
		}
	}
	return nil, false
}

// Returns blockhashes (or nil) that received a +1/3 majority.
// If there exists no such majority, returns nil.
func (vs *VoteSet) OneThirdMajority() (hashes []interface{}) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	oneThirdMajority := (vs.totalVotingPower + uint64(2)) / uint64(3)
	if vs.totalVotes < oneThirdMajority {
		return nil
	}
	for hash, votes := range vs.votesByHash {
		if votes >= oneThirdMajority {
			if hash == "" {
				hashes = append(hashes, nil)
			} else {
				hashes = append(hashes, []byte(hash))
			}
		}
	}
	return hashes
}
