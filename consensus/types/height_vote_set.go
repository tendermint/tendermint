package types

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type RoundVoteSet struct {
	Prevotes   *types.VoteSet
	Precommits *types.VoteSet
}

var (
	ErrGotVoteFromUnwantedRound = errors.New(
		"peer has sent a vote that does not match our round for more than one round",
	)
)

/*
Keeps track of all VoteSets from round 0 to round 'round'.

Also keeps track of up to one RoundVoteSet greater than
'round' from each peer, to facilitate catchup syncing of commits.

A commit is +2/3 precommits for a block at a round,
but which round is not known in advance, so when a peer
provides a precommit for a round greater than mtx.round,
we create a new entry in roundVoteSets but also remember the
peer to prevent abuse.
We let each peer provide us with up to 2 unexpected "catchup" rounds.
One for their LastCommit round, and another for the official commit round.
*/
type HeightVoteSet struct {
	chainID string
	height  int64
	valSet  *types.ValidatorSet

	mtx               sync.Mutex
	round             int                  // max tracked round
	roundVoteSets     map[int]RoundVoteSet // keys: [0...round]
	peerCatchupRounds map[p2p.ID][]int     // keys: peer.ID; values: at most 2 rounds
}

func NewHeightVoteSet(chainID string, height int64, valSet *types.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		chainID: chainID,
	}
	hvs.Reset(height, valSet)
	return hvs
}

func (hvs *HeightVoteSet) Reset(height int64, valSet *types.ValidatorSet) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSets = make(map[int]RoundVoteSet)
	hvs.peerCatchupRounds = make(map[p2p.ID][]int)

	hvs.addRound(0)
	hvs.round = 0
}

func (hvs *HeightVoteSet) Height() int64 {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
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
		panic("SetRound() must increment hvs.round")
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
		panic("addRound() for an existing round")
	}
	// log.Debug("addRound(round)", "round", round)
	prevotes := types.NewVoteSet(hvs.chainID, hvs.height, round, types.PrevoteType, hvs.valSet)
	precommits := types.NewVoteSet(hvs.chainID, hvs.height, round, types.PrecommitType, hvs.valSet)
	hvs.roundVoteSets[round] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// Duplicate votes return added=false, err=nil.
// By convention, peerID is "" if origin is self.
func (hvs *HeightVoteSet) AddVote(vote *types.Vote, peerID p2p.ID) (added bool, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(vote.Type) {
		return
	}
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		if rndz := hvs.peerCatchupRounds[peerID]; len(rndz) < 2 {
			hvs.addRound(vote.Round)
			voteSet = hvs.getVoteSet(vote.Round, vote.Type)
			hvs.peerCatchupRounds[peerID] = append(rndz, vote.Round)
		} else {
			// punish peer
			err = ErrGotVoteFromUnwantedRound
			return
		}
	}
	added, err = voteSet.AddVote(vote)
	return
}

func (hvs *HeightVoteSet) Prevotes(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.PrevoteType)
}

func (hvs *HeightVoteSet) Precommits(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.PrecommitType)
}

// Last round and blockID that has +2/3 prevotes for a particular block or nil.
// Returns -1 if no such round exists.
func (hvs *HeightVoteSet) POLInfo() (polRound int, polBlockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 0; r-- {
		rvs := hvs.getVoteSet(r, types.PrevoteType)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, types.BlockID{}
}

func (hvs *HeightVoteSet) getVoteSet(round int, voteType types.SignedMsgType) *types.VoteSet {
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch voteType {
	case types.PrevoteType:
		return rvs.Prevotes
	case types.PrecommitType:
		return rvs.Precommits
	default:
		panic(fmt.Sprintf("Unexpected vote type %X", voteType))
	}
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
func (hvs *HeightVoteSet) SetPeerMaj23(
	round int,
	voteType types.SignedMsgType,
	peerID p2p.ID,
	blockID types.BlockID) error {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(voteType) {
		return fmt.Errorf("setPeerMaj23: Invalid vote type %X", voteType)
	}
	voteSet := hvs.getVoteSet(round, voteType)
	if voteSet == nil {
		return nil // something we don't know about yet
	}
	return voteSet.SetPeerMaj23(types.P2PID(peerID), blockID)
}

//---------------------------------------------------------
// string and json

func (hvs *HeightVoteSet) String() string {
	return hvs.StringIndented("")
}

func (hvs *HeightVoteSet) StringIndented(indent string) string {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
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
	return fmt.Sprintf(`HeightVoteSet{H:%v R:0~%v
%s  %v
%s}`,
		hvs.height, hvs.round,
		indent, strings.Join(vsStrings, "\n"+indent+"  "),
		indent)
}

func (hvs *HeightVoteSet) MarshalJSON() ([]byte, error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	allVotes := hvs.toAllRoundVotes()
	return cdc.MarshalJSON(allVotes)
}

func (hvs *HeightVoteSet) toAllRoundVotes() []roundVotes {
	totalRounds := hvs.round + 1
	allVotes := make([]roundVotes, totalRounds)
	// rounds 0 ~ hvs.round inclusive
	for round := 0; round < totalRounds; round++ {
		allVotes[round] = roundVotes{
			Round:              round,
			Prevotes:           hvs.roundVoteSets[round].Prevotes.VoteStrings(),
			PrevotesBitArray:   hvs.roundVoteSets[round].Prevotes.BitArrayString(),
			Precommits:         hvs.roundVoteSets[round].Precommits.VoteStrings(),
			PrecommitsBitArray: hvs.roundVoteSets[round].Precommits.BitArrayString(),
		}
	}
	// TODO: all other peer catchup rounds
	return allVotes
}

type roundVotes struct {
	Round              int      `json:"round"`
	Prevotes           []string `json:"prevotes"`
	PrevotesBitArray   string   `json:"prevotes_bit_array"`
	Precommits         []string `json:"precommits"`
	PrecommitsBitArray string   `json:"precommits_bit_array"`
}
