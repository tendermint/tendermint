package consensus

import (
	"strings"
	"sync"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

type RoundVoteSet struct {
	Prevotes   *types.VoteSet
	Precommits *types.VoteSet
}

/*
Keeps track of all VoteSets from round 0 to round 'round'.

Also keeps track of up to one RoundVoteSet greater than
'round' from each peer, to facilitate catchup syncing of commits.

A commit is +2/3 precommits for a block at a round,
but which round is not known in advance, so when a peer
provides a precommit for a round greater than mtx.round,
we create a new entry in roundVoteSets but also remember the
peer to prevent abuse.
*/
type HeightVoteSet struct {
	height int
	valSet *types.ValidatorSet

	mtx               sync.Mutex
	round             int                  // max tracked round
	roundVoteSets     map[int]RoundVoteSet // keys: [0...round]
	peerCatchupRounds map[string]int       // keys: peer.Key; values: round
}

func NewHeightVoteSet(height int, valSet *types.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		height:            height,
		valSet:            valSet,
		roundVoteSets:     make(map[int]RoundVoteSet),
		peerCatchupRounds: make(map[string]int),
	}
	hvs.addRound(0)
	hvs.round = 0
	return hvs
}

func (hvs *HeightVoteSet) Height() int {
	return hvs.height
}

func (hvs *HeightVoteSet) Round() int {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.round
}

// Create more RoundVoteSets up to round.
func (hvs *HeightVoteSet) SetRound(round int) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if hvs.round != 0 && (round < hvs.round+1) {
		PanicSanity("SetRound() must increment hvs.round")
	}
	for r := hvs.round + 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[r]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = round
}

func (hvs *HeightVoteSet) addRound(round int) {
	if _, ok := hvs.roundVoteSets[round]; ok {
		PanicSanity("addRound() for an existing round")
	}
	log.Info("addRound(round)", "round", round)
	prevotes := types.NewVoteSet(hvs.height, round, types.VoteTypePrevote, hvs.valSet)
	precommits := types.NewVoteSet(hvs.height, round, types.VoteTypePrecommit, hvs.valSet)
	hvs.roundVoteSets[round] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// Duplicate votes return added=false, err=nil.
// By convention, peerKey is "" if origin is self.
func (hvs *HeightVoteSet) AddByAddress(address []byte, vote *types.Vote, peerKey string) (added bool, index int, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		if _, ok := hvs.peerCatchupRounds[peerKey]; !ok {
			hvs.addRound(vote.Round)
			hvs.peerCatchupRounds[peerKey] = vote.Round
		} else {
			// Peer has sent a vote that does not match our round,
			// for more than one round.  Bad peer!
			// TODO punish peer.
			log.Warn("Deal with peer giving votes from unwanted rounds")
		}
		return
	}
	added, index, err = voteSet.AddByAddress(address, vote)
	return
}

func (hvs *HeightVoteSet) Prevotes(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.VoteTypePrevote)
}

func (hvs *HeightVoteSet) Precommits(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.VoteTypePrecommit)
}

// Last round that has +2/3 prevotes for a particular block or nik.
// Returns -1 if no such round exists.
func (hvs *HeightVoteSet) POLRound() int {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 0; r-- {
		if hvs.getVoteSet(r, types.VoteTypePrevote).HasTwoThirdsMajority() {
			return r
		}
	}
	return -1
}

func (hvs *HeightVoteSet) getVoteSet(round int, type_ byte) *types.VoteSet {
	log.Info("getVoteSet(round)", "round", round, "type", type_)
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
		PanicSanity(Fmt("Unexpected vote type %X", type_))
		return nil
	}
}

func (hvs *HeightVoteSet) String() string {
	return hvs.StringIndented("")
}

func (hvs *HeightVoteSet) StringIndented(indent string) string {
	vsStrings := make([]string, 0, (len(hvs.roundVoteSets)+1)*2)
	// rounds 0 ~ hvs.round inclusive
	for round := 0; round <= hvs.round; round++ {
		voteSetString := hvs.roundVoteSets[round].Prevotes.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = hvs.roundVoteSets[round].Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	// all other peer catchup rounds
	for round, roundVoteSet := range hvs.roundVoteSets {
		if round <= hvs.round {
			continue
		}
		voteSetString := roundVoteSet.Prevotes.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = roundVoteSet.Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	return Fmt(`HeightVoteSet{H:%v R:0~%v
%s  %v
%s}`,
		hvs.height, hvs.round,
		indent, strings.Join(vsStrings, "\n"+indent+"  "),
		indent)
}
