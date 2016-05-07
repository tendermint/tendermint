package types

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
)

// VoteSet helps collect signatures from validators at each height+round
// for a predefined vote type.
// Note that there three kinds of votes: prevotes, precommits, and commits.
// A commit of prior rounds can be added added in lieu of votes/precommits.
// NOTE: Assumes that the sum total of voting power does not exceed MaxUInt64.
type VoteSet struct {
	height int
	round  int
	type_  byte

	mtx              sync.Mutex
	valSet           *ValidatorSet
	votes            []*Vote          // validator index -> vote
	votesBitArray    *BitArray        // validator index -> has vote?
	votesByBlock     map[string]int64 // string(blockHash)+string(blockParts) -> vote sum.
	totalVotes       int64
	maj23Hash        []byte
	maj23PartsHeader PartSetHeader
	maj23Exists      bool
}

// Constructs a new VoteSet struct used to accumulate votes for given height/round.
func NewVoteSet(height int, round int, type_ byte, valSet *ValidatorSet) *VoteSet {
	if height == 0 {
		PanicSanity("Cannot make VoteSet for height == 0, doesn't make sense.")
	}
	return &VoteSet{
		height:        height,
		round:         round,
		type_:         type_,
		valSet:        valSet,
		votes:         make([]*Vote, valSet.Size()),
		votesBitArray: NewBitArray(valSet.Size()),
		votesByBlock:  make(map[string]int64),
		totalVotes:    0,
	}
}

func (voteSet *VoteSet) Height() int {
	if voteSet == nil {
		return 0
	} else {
		return voteSet.height
	}
}

func (voteSet *VoteSet) Round() int {
	if voteSet == nil {
		return -1
	} else {
		return voteSet.round
	}
}

func (voteSet *VoteSet) Type() byte {
	if voteSet == nil {
		return 0x00
	} else {
		return voteSet.type_
	}
}

func (voteSet *VoteSet) Size() int {
	if voteSet == nil {
		return 0
	} else {
		return voteSet.valSet.Size()
	}
}

// Returns added=true, index if vote was added
// Otherwise returns err=ErrVote[UnexpectedStep|InvalidAccount|InvalidSignature|InvalidBlockHash|ConflictingSignature]
// Duplicate votes return added=false, err=nil.
// NOTE: vote should not be mutated after adding.
func (voteSet *VoteSet) AddByIndex(valIndex int, vote *Vote) (added bool, address []byte, err error) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	return voteSet.addByIndex(valIndex, vote)
}

// Returns added=true, index if vote was added
// Otherwise returns err=ErrVote[UnexpectedStep|InvalidAccount|InvalidSignature|InvalidBlockHash|ConflictingSignature]
// Duplicate votes return added=false, err=nil.
// NOTE: vote should not be mutated after adding.
func (voteSet *VoteSet) AddByAddress(address []byte, vote *Vote) (added bool, index int, err error) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	// Ensure that signer is a validator.
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		return false, 0, ErrVoteInvalidAccount
	}

	return voteSet.addVote(val, valIndex, vote)
}

func (voteSet *VoteSet) addByIndex(valIndex int, vote *Vote) (added bool, address []byte, err error) {
	// Ensure that signer is a validator.
	address, val := voteSet.valSet.GetByIndex(valIndex)
	if val == nil {
		return false, nil, ErrVoteInvalidAccount
	}

	added, _, err = voteSet.addVote(val, valIndex, vote)
	return
}

func (voteSet *VoteSet) addVote(val *Validator, valIndex int, vote *Vote) (bool, int, error) {

	// Make sure the step matches. (or that vote is commit && round < voteSet.round)
	if (vote.Height != voteSet.height) ||
		(vote.Round != voteSet.round) ||
		(vote.Type != voteSet.type_) {
		return false, 0, ErrVoteUnexpectedStep
	}

	var doubleSign bool
	// If vote already exists, return false.
	if existingVote := voteSet.votes[valIndex]; existingVote != nil {
		if bytes.Equal(existingVote.BlockHash, vote.BlockHash) {
			return false, valIndex, nil
		} else {
			doubleSign = true
			log.Warn("Double sign detected!", "height", vote.Height, "round", vote.Round, "type", vote.Type, "index", valIndex)

			// if its a preovte, or we already have a majority, return the double sign error.
			// else, we may need to replace the vote if its part of a commit
			// so we let it fall through
			if vote.Type == VoteTypePrevote || voteSet.maj23Exists {
				return false, valIndex, &ErrVoteConflictingSignature{
					VoteA: existingVote,
					VoteB: vote,
				}
			}

			// subtract voting power from the existing vote
			blockKey := string(existingVote.BlockHash) + string(wire.BinaryBytes(existingVote.BlockPartsHeader))
			totalBlockHashVotes := voteSet.votesByBlock[blockKey] - val.VotingPower
			voteSet.votesByBlock[blockKey] = totalBlockHashVotes
		}
	}

	// Check signature.
	if !val.PubKey.VerifyBytes(SignBytes(config.GetString("chain_id"), vote), vote.Signature) {
		// Bad signature.
		return false, 0, ErrVoteInvalidSignature
	}

	// Add vote.
	voteSet.votes[valIndex] = vote
	voteSet.votesBitArray.SetIndex(valIndex, true)
	blockKey := string(vote.BlockHash) + string(wire.BinaryBytes(vote.BlockPartsHeader))
	totalBlockHashVotes := voteSet.votesByBlock[blockKey] + val.VotingPower
	voteSet.votesByBlock[blockKey] = totalBlockHashVotes
	if !doubleSign {
		voteSet.totalVotes += val.VotingPower
	}

	// If we just nudged it up to two thirds majority, add it.
	if totalBlockHashVotes > voteSet.valSet.TotalVotingPower()*2/3 &&
		(totalBlockHashVotes-val.VotingPower) <= voteSet.valSet.TotalVotingPower()*2/3 {
		voteSet.maj23Hash = vote.BlockHash
		voteSet.maj23PartsHeader = vote.BlockPartsHeader
		voteSet.maj23Exists = true
	}

	if doubleSign {
		/*
			return true, valIndex, &ErrVoteConflictingSignature{
				VoteA: existingVote,
				VoteB: vote,
			}*/
	}

	return true, valIndex, nil
}

func (voteSet *VoteSet) BitArray() *BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) GetByIndex(valIndex int) *Vote {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) GetByAddress(address []byte) *Vote {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		PanicSanity("GetByAddress(address) returned nil")
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

func (voteSet *VoteSet) IsCommit() bool {
	if voteSet == nil {
		return false
	}
	if voteSet.type_ != VoteTypePrecommit {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return len(voteSet.maj23Hash) > 0
}

func (voteSet *VoteSet) HasTwoThirdsAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.totalVotes > voteSet.valSet.TotalVotingPower()*2/3
}

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, false).
func (voteSet *VoteSet) TwoThirdsMajority() (hash []byte, parts PartSetHeader, ok bool) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23Exists {
		return voteSet.maj23Hash, voteSet.maj23PartsHeader, true
	} else {
		return nil, PartSetHeader{}, false
	}
}

func (voteSet *VoteSet) String() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	return voteSet.StringIndented("")
}

func (voteSet *VoteSet) StringIndented(indent string) string {
	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = "nil-Vote"
		} else {
			voteStrings[i] = vote.String()
		}
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

func (voteSet *VoteSet) StringShort() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v +2/3:%v %v}`,
		voteSet.height, voteSet.round, voteSet.type_, voteSet.maj23Exists, voteSet.votesBitArray)
}

//--------------------------------------------------------------------------------
// Commit

func (voteSet *VoteSet) MakeCommit() *Commit {
	if voteSet.type_ != VoteTypePrecommit {
		PanicSanity("Cannot MakeCommit() unless VoteSet.Type is VoteTypePrecommit")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if len(voteSet.maj23Hash) == 0 {
		PanicSanity("Cannot MakeCommit() unless a blockhash has +2/3")
	}
	precommits := make([]*Vote, voteSet.valSet.Size())
	voteSet.valSet.Iterate(func(valIndex int, val *Validator) bool {
		vote := voteSet.votes[valIndex]
		if vote == nil {
			return false
		}
		if !bytes.Equal(vote.BlockHash, voteSet.maj23Hash) {
			return false
		}
		if !vote.BlockPartsHeader.Equals(voteSet.maj23PartsHeader) {
			return false
		}
		precommits[valIndex] = vote
		return false
	})
	return &Commit{
		Precommits: precommits,
	}
}
