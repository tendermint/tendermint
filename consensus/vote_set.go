package consensus

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/state"
)

// VoteSet helps collect signatures from validators at each height+round
// for a predefined vote type.
// Note that there three kinds of votes: prevotes, precommits, and commits.
// A commit of prior rounds can be added added in lieu of votes/precommits.
// TODO: test majority calculations etc.
// NOTE: assumes that the sum total of voting power does not exceed MaxUInt64.
type VoteSet struct {
	height uint32
	round  uint16
	type_  byte

	mtx               sync.Mutex
	vset              *state.ValidatorSet
	votes             map[uint64]*Vote
	votesBitArray     BitArray
	votesByBlockHash  map[string]uint64
	totalVotes        uint64
	twoThirdsMajority []byte
	twoThirdsExists   bool
}

// Constructs a new VoteSet struct used to accumulate votes for each round.
func NewVoteSet(height uint32, round uint16, type_ byte, vset *state.ValidatorSet) *VoteSet {
	if type_ == VoteTypeCommit && round != 0 {
		panic("Expected round 0 for commit vote set")
	}
	return &VoteSet{
		height:           height,
		round:            round,
		type_:            type_,
		vset:             vset,
		votes:            make(map[uint64]*Vote, vset.Size()),
		votesBitArray:    NewBitArray(vset.Size()),
		votesByBlockHash: make(map[string]uint64),
		totalVotes:       0,
	}
}

// True if added, false if not.
// Returns ErrVote[UnexpectedStep|InvalidAccount|InvalidSignature|InvalidBlockHash|ConflictingSignature]
// NOTE: vote should not be mutated after adding.
func (vs *VoteSet) Add(vote *Vote) (bool, error) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()

	// Make sure the step matches. (or that vote is commit && round < vs.round)
	if vote.Height != vs.height ||
		(vote.Type != VoteTypeCommit && vote.Round != vs.round) ||
		(vote.Type != VoteTypeCommit && vote.Type != vs.type_) ||
		(vote.Type == VoteTypeCommit && vs.type_ != VoteTypeCommit && vote.Round >= vs.round) {
		return false, ErrVoteUnexpectedStep
	}

	// Ensure that signer is a validator.
	_, val := vs.vset.GetById(vote.SignerId)
	if val == nil {
		return false, ErrVoteInvalidAccount
	}

	// Check signature.
	if !val.Verify(vote) {
		// Bad signature.
		return false, ErrVoteInvalidSignature
	}

	return vs.addVote(vote)
}

func (vs *VoteSet) addVote(vote *Vote) (bool, error) {
	// If vote already exists, return false.
	if existingVote, ok := vs.votes[vote.SignerId]; ok {
		if bytes.Equal(existingVote.BlockHash, vote.BlockHash) {
			return false, nil
		} else {
			return false, ErrVoteConflictingSignature
		}
	}

	// Add vote.
	vs.votes[vote.SignerId] = vote
	voterIndex, val := vs.vset.GetById(vote.SignerId)
	if val == nil {
		return false, ErrVoteInvalidAccount
	}
	vs.votesBitArray.SetIndex(uint(voterIndex), true)
	totalBlockHashVotes := vs.votesByBlockHash[string(vote.BlockHash)] + val.VotingPower
	vs.votesByBlockHash[string(vote.BlockHash)] = totalBlockHashVotes
	vs.totalVotes += val.VotingPower

	// If we just nudged it up to two thirds majority, add it.
	if totalBlockHashVotes > vs.vset.TotalVotingPower()*2/3 &&
		(totalBlockHashVotes-val.VotingPower) <= vs.vset.TotalVotingPower()*2/3 {
		vs.twoThirdsMajority = vote.BlockHash
		vs.twoThirdsExists = true
	}

	return true, nil
}

// Assumes that commits VoteSet is valid.
func (vs *VoteSet) AddFromCommits(commits *VoteSet) {
	commitVotes := commits.AllVotes()
	for _, commit := range commitVotes {
		if commit.Round < vs.round {
			vs.addVote(commit)
		}
	}
}

func (vs *VoteSet) BitArray() BitArray {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	return vs.votesBitArray.Copy()
}

func (vs *VoteSet) Get(id uint64) *Vote {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	return vs.votes[id]
}

func (vs *VoteSet) AllVotes() []*Vote {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	votes := []*Vote{}
	for _, vote := range vs.votes {
		votes = append(votes, vote)
	}
	return votes
}

func (vs *VoteSet) HasTwoThirdsMajority() bool {
	if vs == nil {
		return false
	}
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	return vs.twoThirdsExists
}

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, false).
func (vs *VoteSet) TwoThirdsMajority() (hash []byte, ok bool) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	if vs.twoThirdsExists {
		return vs.twoThirdsMajority, true
	} else {
		return nil, false
	}
}

func (vs *VoteSet) MakePOL() *POL {
	if vs.type_ != VoteTypePrevote {
		panic("Cannot MakePOL() unless VoteSet.Type is VoteTypePrevote")
	}
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	if !vs.twoThirdsExists {
		return nil
	}
	majHash := vs.twoThirdsMajority // hash may be nil.
	pol := &POL{
		Height:    vs.height,
		Round:     vs.round,
		BlockHash: majHash,
	}
	for _, vote := range vs.votes {
		if bytes.Equal(vote.BlockHash, majHash) {
			if vote.Type == VoteTypePrevote {
				pol.Votes = append(pol.Votes, vote.Signature)
			} else if vote.Type == VoteTypeCommit {
				pol.Commits = append(pol.Votes, vote.Signature)
				pol.CommitRounds = append(pol.CommitRounds, vote.Round)
			} else {
				Panicf("Unexpected vote type %X", vote.Type)
			}
		}
	}
	return pol
}

func (vs *VoteSet) MakeValidation() Validation {
	if vs.type_ != VoteTypeCommit {
		panic("Cannot MakeValidation() unless VoteSet.Type is VoteTypePrevote")
	}
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	if len(vs.twoThirdsMajority) == 0 {
		panic("Cannot MakeValidation() unless a blockhash has +2/3")
	}
	sigs := []Signature{}
	for _, vote := range vs.votes {
		if !bytes.Equal(vote.BlockHash, vs.twoThirdsMajority) {
			continue
		}
		sigs = append(sigs, vote.Signature)
	}
	return Validation{
		Signatures: sigs,
	}
}

func (vs *VoteSet) String() string {
	return vs.StringWithIndent("")
}

func (vs *VoteSet) StringWithIndent(indent string) string {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()

	voteStrings := make([]string, len(vs.votes))
	counter := 0
	for _, vote := range vs.votes {
		voteStrings[counter] = vote.String()
		counter++
	}
	return fmt.Sprintf(`VoteSet{
%s  H:%v R:%v T:%v
%s  %v
%s  %v
%s}`,
		indent, vs.height, vs.round, vs.type_,
		indent, strings.Join(voteStrings, "\n"+indent+"  "),
		indent, vs.votesBitArray,
		indent)
}

func (vs *VoteSet) Description() string {
	if vs == nil {
		return "nil-VoteSet"
	}
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v %v}`,
		vs.height, vs.round, vs.type_, vs.votesBitArray)
}
