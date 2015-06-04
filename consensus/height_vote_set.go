package consensus

import (
	"strings"
	"sync"

	. "github.com/tendermint/tendermint/common"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type RoundVoteSet struct {
	Prevotes   *VoteSet
	Precommits *VoteSet
}

// Keeps track of VoteSets for all the rounds of a height.
// We add the commit votes to all the affected rounds,
// and for new rounds carry over the commit set. Commits have
// an associated round, so the performance hit won't be O(rounds).
type HeightVoteSet struct {
	height uint
	valSet *sm.ValidatorSet

	mtx           sync.Mutex
	round         uint                  // max tracked round
	roundVoteSets map[uint]RoundVoteSet // keys: [0...round]
	commits       *VoteSet              // all commits for height
}

func NewHeightVoteSet(height uint, valSet *sm.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		height:        height,
		valSet:        valSet,
		roundVoteSets: make(map[uint]RoundVoteSet),
		commits:       NewVoteSet(height, 0, types.VoteTypeCommit, valSet),
	}
	hvs.SetRound(0)
	return hvs
}

func (hvs *HeightVoteSet) Height() uint {
	return hvs.height
}

func (hvs *HeightVoteSet) Round() uint {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.round
}

// Create more RoundVoteSets up to round with all commits carried over.
func (hvs *HeightVoteSet) SetRound(round uint) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if hvs.round != 0 && (round < hvs.round+1) {
		panic("SetRound() must increment hvs.round")
	}
	for r := hvs.round + 1; r <= round; r++ {
		prevotes := NewVoteSet(hvs.height, r, types.VoteTypePrevote, hvs.valSet)
		prevotes.AddFromCommits(hvs.commits)
		precommits := NewVoteSet(hvs.height, r, types.VoteTypePrecommit, hvs.valSet)
		precommits.AddFromCommits(hvs.commits)
		hvs.roundVoteSets[r] = RoundVoteSet{
			Prevotes:   prevotes,
			Precommits: precommits,
		}
	}
	hvs.round = round
}

// CONTRACT: if err == nil, added == true
func (hvs *HeightVoteSet) AddByAddress(address []byte, vote *types.Vote) (added bool, index uint, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		return
	}
	added, index, err = voteSet.AddByAddress(address, vote)
	if err != nil {
		return
	}
	// If vote is commit, also add to all prevote/precommit for future rounds.
	if vote.Type == types.VoteTypeCommit {
		for round := vote.Round + 1; round <= hvs.round; round++ {
			voteSet := hvs.getVoteSet(round, types.VoteTypePrevote)
			_, _, err = voteSet.AddByAddress(address, vote)
			if err != nil {
				// TODO slash for prevote after commit
				log.Warn("Prevote after commit", "address", address, "vote", vote)
			}
			voteSet = hvs.getVoteSet(round, types.VoteTypePrecommit)
			_, _, err = voteSet.AddByAddress(address, vote)
			if err != nil {
				// TODO slash for prevote after commit
				log.Warn("Prevote after commit", "address", address, "vote", vote)
			}
		}
	}
	return
}

func (hvs *HeightVoteSet) GetVoteSet(round uint, type_ byte) *VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, type_)
}

func (hvs *HeightVoteSet) getVoteSet(round uint, type_ byte) *VoteSet {
	if type_ == types.VoteTypeCommit {
		return hvs.commits
	}
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch type_ {
	case types.VoteTypePrevote:
		return rvs.Prevotes
	case types.VoteTypePrecommit:
		return rvs.Precommits
	default:
		panic(Fmt("Unexpected vote type %X", type_))
	}
}

func (hvs *HeightVoteSet) String() string {
	return hvs.StringIndented("")
}

func (hvs *HeightVoteSet) StringIndented(indent string) string {
	vsStrings := make([]string, 0, hvs.round*2+1)
	vsStrings = append(vsStrings, hvs.commits.StringShort())
	for round := uint(0); round <= hvs.round; round++ {
		voteSetString := hvs.roundVoteSets[round].Prevotes.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = hvs.roundVoteSets[round].Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	return Fmt(`HeightVoteSet{H:%v R:0~%v
%s  %v
%s}`,
		indent, strings.Join(vsStrings, "\n"+indent+"  "),
		indent)
}
