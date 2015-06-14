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

/*
Keeps track of all VoteSets from round 0 to round 'round'.

Also keeps track of up to one RoundVoteSet greater than
'round' from each peer, to facilitate fast-forward syncing.
A commit is +2/3 precommits for a block at a round,
but which round is not known in advance, so when a peer
provides a precommit for a round greater than mtx.round,
we create a new entry in roundVoteSets but also remember the
peer to prevent abuse.
*/
type HeightVoteSet struct {
	height uint
	valSet *sm.ValidatorSet

	mtx             sync.Mutex
	round           uint                  // max tracked round
	roundVoteSets   map[uint]RoundVoteSet // keys: [0...round]
	peerFastForward map[string]uint       // keys: peer.Key; values: round
}

func NewHeightVoteSet(height uint, valSet *sm.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		height:         height,
		valSet:         valSet,
		roundVoteSets:  make(map[uint]RoundVoteSet),
		peerFFPrevotes: make(map[string]*VoteSet),
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

// Create more RoundVoteSets up to round.
func (hvs *HeightVoteSet) SetRound(round uint) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if hvs.round != 0 && (round < hvs.round+1) {
		panic("SetRound() must increment hvs.round")
	}
	for r := hvs.round + 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSet[r]; ok {
			continue // Already exists because peerFastForward.
		}
		hvs.addRound(round)
	}
	hvs.round = round
}

func (hvs *HeightVoteSet) addRound(round uint) {
	if _, ok := hvs.roundVoteSet[r]; ok {
		panic("addRound() for an existing round")
	}
	prevotes := NewVoteSet(hvs.height, r, types.VoteTypePrevote, hvs.valSet)
	precommits := NewVoteSet(hvs.height, r, types.VoteTypePrecommit, hvs.valSet)
	hvs.roundVoteSets[r] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// CONTRACT: if err == nil, added == true
func (hvs *HeightVoteSet) AddByAddress(address []byte, vote *types.Vote, peer string) (added bool, index uint, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		if _, ok := hvs.peerFastForward[peer]; !ok {
			hvs.addRound(vote.Round)
			hvs.peerFastForwards[peer] = vote.Round
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

func (hvs *HeightVoteSet) Prevotes(round uint) *VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.VoteTypePrevote)
}

func (hvs *HeightVoteSet) Precommits(round uint) *VoteSet {
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
			return int(r)
		}
	}
	return -1
}

func (hvs *HeightVoteSet) getVoteSet(round uint, type_ byte) *VoteSet {
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
	vsStrings := make([]string, 0, (len(hvs.roundVoteSets)+1)*2)
	// rounds 0 ~ hvs.round inclusive
	for round := uint(0); round <= hvs.round; round++ {
		voteSetString := hvs.roundVoteSets[round].Prevotes.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = hvs.roundVoteSets[round].Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	// all other peer fast-forward rounds
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
