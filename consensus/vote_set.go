package consensus

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/state"
)

// VoteSet helps collect signatures from validators at each height+round
// for a predefined vote type.
// Note that there three kinds of votes: prevotes, precommits, and commits.
// A commit of prior rounds can be added added in lieu of votes/precommits.
// NOTE: Assumes that the sum total of voting power does not exceed MaxUInt64.
type VoteSet struct {
	height uint
	round  uint
	type_  byte

	mtx           sync.Mutex
	valSet        *state.ValidatorSet
	votes         []*Vote           // validator index -> vote
	votesBitArray BitArray          // validator index -> has vote?
	votesByBlock  map[string]uint64 // string(blockHash)+string(blockParts) -> vote sum.
	totalVotes    uint64
	maj23Hash     []byte
	maj23Parts    PartSetHeader
	maj23Exists   bool
}

// Constructs a new VoteSet struct used to accumulate votes for each round.
func NewVoteSet(height uint, round uint, type_ byte, valSet *state.ValidatorSet) *VoteSet {
	if height == 0 {
		panic("Cannot make VoteSet for height == 0, doesn't make sense.")
	}
	if type_ == VoteTypeCommit && round != 0 {
		panic("Expected round 0 for commit vote set")
	}
	return &VoteSet{
		height:        height,
		round:         round,
		type_:         type_,
		valSet:        valSet,
		votes:         make([]*Vote, valSet.Size()),
		votesBitArray: NewBitArray(valSet.Size()),
		votesByBlock:  make(map[string]uint64),
		totalVotes:    0,
	}
}

func (voteSet *VoteSet) Size() uint {
	if voteSet == nil {
		return 0
	} else {
		return voteSet.valSet.Size()
	}
}

// True if added, false if not.
// Returns ErrVote[UnexpectedStep|InvalidAccount|InvalidSignature|InvalidBlockHash|ConflictingSignature]
// NOTE: vote should not be mutated after adding.
func (voteSet *VoteSet) Add(address []byte, vote *Vote) (bool, uint, error) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	// Make sure the step matches. (or that vote is commit && round < voteSet.round)
	if vote.Height != voteSet.height ||
		(vote.Type != VoteTypeCommit && vote.Round != voteSet.round) ||
		(vote.Type != VoteTypeCommit && vote.Type != voteSet.type_) ||
		(vote.Type == VoteTypeCommit && voteSet.type_ != VoteTypeCommit && vote.Round >= voteSet.round) {
		return false, 0, ErrVoteUnexpectedStep
	}

	// Ensure that signer is a validator.
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		return false, 0, ErrVoteInvalidAccount
	}

	// Check signature.
	if !val.PubKey.VerifyBytes(SignBytes(vote), vote.Signature) {
		// Bad signature.
		return false, 0, ErrVoteInvalidSignature
	}

	return voteSet.addVote(valIndex, vote)
}

func (voteSet *VoteSet) addVote(valIndex uint, vote *Vote) (bool, uint, error) {
	// If vote already exists, return false.
	if existingVote := voteSet.votes[valIndex]; existingVote != nil {
		if bytes.Equal(existingVote.BlockHash, vote.BlockHash) {
			return false, 0, nil
		} else {
			return false, 0, ErrVoteConflictingSignature
		}
	}

	// Add vote.
	_, val := voteSet.valSet.GetByIndex(valIndex)
	if val == nil {
		panic(fmt.Sprintf("Missing validator for index %v", valIndex))
	}
	voteSet.votes[valIndex] = vote
	voteSet.votesBitArray.SetIndex(valIndex, true)
	blockKey := string(vote.BlockHash) + string(BinaryBytes(vote.BlockParts))
	totalBlockHashVotes := voteSet.votesByBlock[blockKey] + val.VotingPower
	voteSet.votesByBlock[blockKey] = totalBlockHashVotes
	voteSet.totalVotes += val.VotingPower

	// If we just nudged it up to two thirds majority, add it.
	if totalBlockHashVotes > voteSet.valSet.TotalVotingPower()*2/3 &&
		(totalBlockHashVotes-val.VotingPower) <= voteSet.valSet.TotalVotingPower()*2/3 {
		voteSet.maj23Hash = vote.BlockHash
		voteSet.maj23Parts = vote.BlockParts
		voteSet.maj23Exists = true
	}

	return true, valIndex, nil
}

// Assumes that commits VoteSet is valid.
func (voteSet *VoteSet) AddFromCommits(commits *VoteSet) {
	for valIndex, commit := range commits.votes {
		if commit.Round < voteSet.round {
			voteSet.addVote(uint(valIndex), commit)
		}
	}
}

func (voteSet *VoteSet) BitArray() BitArray {
	if voteSet == nil {
		return BitArray{}
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) GetByIndex(valIndex uint) *Vote {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) GetByAddress(address []byte) *Vote {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		panic("GetByAddress(address) returned nil")
	}
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) HasTwoThirdsMajority() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23Exists
}

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, false).
func (voteSet *VoteSet) TwoThirdsMajority() (hash []byte, parts PartSetHeader, ok bool) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23Exists {
		return voteSet.maj23Hash, voteSet.maj23Parts, true
	} else {
		return nil, PartSetHeader{}, false
	}
}

func (voteSet *VoteSet) MakePOL() *POL {
	if voteSet.type_ != VoteTypePrevote {
		panic("Cannot MakePOL() unless VoteSet.Type is VoteTypePrevote")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if !voteSet.maj23Exists {
		return nil
	}
	pol := &POL{
		Height:     voteSet.height,
		Round:      voteSet.round,
		BlockHash:  voteSet.maj23Hash,
		BlockParts: voteSet.maj23Parts,
		Votes:      make([]POLVoteSignature, voteSet.valSet.Size()),
	}
	for valIndex, vote := range voteSet.votes {
		if !bytes.Equal(vote.BlockHash, voteSet.maj23Hash) {
			continue
		}
		if !vote.BlockParts.Equals(voteSet.maj23Parts) {
			continue
		}
		pol.Votes[valIndex] = POLVoteSignature{
			Round:     vote.Round,
			Signature: vote.Signature,
		}
	}
	return pol
}

func (voteSet *VoteSet) MakeValidation() *Validation {
	if voteSet.type_ != VoteTypeCommit {
		panic("Cannot MakeValidation() unless VoteSet.Type is VoteTypeCommit")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if len(voteSet.maj23Hash) == 0 {
		panic("Cannot MakeValidation() unless a blockhash has +2/3")
	}
	commits := make([]Commit, voteSet.valSet.Size())
	voteSet.valSet.Iterate(func(valIndex uint, val *state.Validator) bool {
		vote := voteSet.votes[valIndex]
		if vote == nil {
			return false
		}
		if !bytes.Equal(vote.BlockHash, voteSet.maj23Hash) {
			return false
		}
		if !vote.BlockParts.Equals(voteSet.maj23Parts) {
			return false
		}
		commits[valIndex] = Commit{vote.Round, vote.Signature}
		return false
	})
	return &Validation{
		Commits: commits,
	}
}

func (voteSet *VoteSet) String() string {
	return voteSet.StringWithIndent("")
}

func (voteSet *VoteSet) StringWithIndent(indent string) string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	voteStrings := make([]string, len(voteSet.votes))
	counter := 0
	for _, vote := range voteSet.votes {
		voteStrings[counter] = vote.String()
		counter++
	}
	return fmt.Sprintf(`VoteSet{
%s  H:%v R:%v T:%v
%s  %v
%s  %v
%s}`,
		indent, voteSet.height, voteSet.round, voteSet.type_,
		indent, strings.Join(voteStrings, "\n"+indent+"  "),
		indent, voteSet.votesBitArray,
		indent)
}

func (voteSet *VoteSet) Description() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v %v}`,
		voteSet.height, voteSet.round, voteSet.type_, voteSet.votesBitArray)
}
