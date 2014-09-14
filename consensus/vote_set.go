package consensus

import (
	"bytes"

	"sync"
	"time"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/state"
)

// VoteSet helps collect signatures from validators at each height+round
// for a predefined vote type.
// Note that there three kinds of votes: (bare) votes, precommits, and commits.
// A commit of prior rounds can be added added in lieu of votes/precommits.
// TODO: test majority calculations etc.
type VoteSet struct {
	height uint32
	round  uint16
	type_  byte

	mtx                 sync.Mutex
	vset                *ValidatorSet
	votes               map[uint64]*Vote
	votesBitArray       BitArray
	votesByBlockHash    map[string]uint64
	totalVotes          uint64
	twoThirdsMajority   []byte
	twoThirdsCommitTime time.Time
}

// Constructs a new VoteSet struct used to accumulate votes for each round.
func NewVoteSet(height uint32, round uint16, type_ byte, vset *ValidatorSet) *VoteSet {
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
// Returns ErrVote[UnexpectedPhase|InvalidAccount|InvalidSignature|InvalidBlockHash|ConflictingSignature]
func (vs *VoteSet) AddVote(vote *Vote) (bool, error) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()

	// Make sure the phase matches. (or that vote is commit && round < vs.round)
	if vote.Height != vs.height ||
		(vote.Type != VoteTypeCommit && vote.Round != vs.round) ||
		(vote.Type != VoteTypeCommit && vote.Type != vs.type_) ||
		(vote.Type == VoteTypeCommit && vs.type_ != VoteTypeCommit && vote.Round >= vs.round) {
		return false, ErrVoteUnexpectedPhase
	}

	// Ensure that signer is a validator.
	val := vs.vset.GetById(vote.SignerId)
	if val == nil {
		return false, ErrVoteInvalidAccount
	}

	// Check signature.
	if !val.Verify(vote.GenDocument(), vote.Signature) {
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
	voterIndex, ok := vs.vset.GetIndexById(vote.SignerId)
	if !ok {
		return false, ErrVoteInvalidAccount
	}
	vs.votesBitArray.SetIndex(uint(voterIndex), true)
	val := vs.vset.GetById(vote.SignerId)
	totalBlockHashVotes := vs.votesByBlockHash[string(vote.BlockHash)] + val.VotingPower
	vs.votesByBlockHash[string(vote.BlockHash)] = totalBlockHashVotes
	vs.totalVotes += val.VotingPower

	// If we just nudged it up to two thirds majority, add it.
	if totalBlockHashVotes > vs.vset.TotalVotingPower()*2/3 &&
		(totalBlockHashVotes-val.VotingPower) <= vs.vset.TotalVotingPower()*2/3 {
		vs.twoThirdsMajority = vote.BlockHash
		vs.twoThirdsCommitTime = time.Now()
	}

	return true, nil
}

// Assumes that commits VoteSet is valid.
func (vs *VoteSet) AddVotesFromCommits(commits *VoteSet) {
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

func (vs *VoteSet) GetVote(id uint64) *Vote {
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

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, false).
func (vs *VoteSet) TwoThirdsMajority() (hash []byte, commitTime time.Time, ok bool) {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	if vs.twoThirdsCommitTime.IsZero() {
		return nil, time.Time{}, false
	}
	return vs.twoThirdsMajority, vs.twoThirdsCommitTime, true
}

func (vs *VoteSet) MakePOL() *POL {
	vs.mtx.Lock()
	defer vs.mtx.Unlock()
	if vs.twoThirdsCommitTime.IsZero() {
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
			if vote.Type == VoteTypeBare {
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
