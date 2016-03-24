package consensus

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-events"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/config/tendermint_test"
	mempl "github.com/tendermint/tendermint/mempool"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmspcli "github.com/tendermint/tmsp/client"

	"github.com/tendermint/tmsp/example/counter"
)

var chainID string

var ensureTimeout = time.Duration(2)

func init() {
	tendermint_test.ResetConfig("consensus_common_test")
	chainID = config.GetString("chain_id")
}

type validatorStub struct {
	Height int
	Round  int
	*types.PrivValidator
}

func NewValidatorStub(privValidator *types.PrivValidator) *validatorStub {
	return &validatorStub{
		PrivValidator: privValidator,
	}
}

func (vs *validatorStub) signVote(voteType byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	vote := &types.Vote{
		Height:           vs.Height,
		Round:            vs.Round,
		Type:             voteType,
		BlockHash:        hash,
		BlockPartsHeader: header,
	}
	err := vs.PrivValidator.SignVote(chainID, vote)
	return vote, err
}

// convenienve function for testing
func signVote(vs *validatorStub, voteType byte, hash []byte, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

// create proposal block from cs1 but sign it with vs
func decideProposal(cs1 *ConsensusState, cs2 *validatorStub, height, round int) (proposal *types.Proposal, block *types.Block) {
	block, blockParts := cs1.createProposalBlock()
	if block == nil { // on error
		panic("error creating proposal block")
	}

	// Make proposal
	proposal = types.NewProposal(height, round, blockParts.Header(), cs1.Votes.POLRound())
	if err := cs2.SignProposal(chainID, proposal); err != nil {
		panic(err)
	}
	return
}

//-------------------------------------------------------------------------------
// utils

/*
func nilRound(t *testing.T, cs1 *ConsensusState, vss ...*validatorStub) {
	cs1.mtx.Lock()
	height, round := cs1.Height, cs1.Round
	cs1.mtx.Unlock()

	waitFor(t, cs1, height, round, RoundStepPrevote)

	signAddVoteToFromMany(types.VoteTypePrevote, cs1, nil, cs1.ProposalBlockParts.Header(), vss...)

	waitFor(t, cs1, height, round, RoundStepPrecommit)

	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, cs1.ProposalBlockParts.Header(), vss...)

	waitFor(t, cs1, height, round+1, RoundStepNewRound)
}
*/

// NOTE: this switches the propser as far as `perspectiveOf` is concerned,
// but for simplicity we return a block it generated.
func changeProposer(t *testing.T, perspectiveOf *ConsensusState, newProposer *validatorStub) *types.Block {
	_, v1 := perspectiveOf.Validators.GetByAddress(perspectiveOf.privValidator.Address)
	v1.Accum, v1.VotingPower = 0, 0
	if updated := perspectiveOf.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}
	_, v2 := perspectiveOf.Validators.GetByAddress(newProposer.Address)
	v2.Accum, v2.VotingPower = 100, 100
	if updated := perspectiveOf.Validators.Update(v2); !updated {
		t.Fatal("failed to update validator")
	}

	// make the proposal
	propBlock, _ := perspectiveOf.createProposalBlock()
	if propBlock == nil {
		t.Fatal("Failed to create proposal block with cs2")
	}
	return propBlock
}

func fixVotingPower(t *testing.T, cs1 *ConsensusState, addr2 []byte) {
	_, v1 := cs1.Validators.GetByAddress(cs1.privValidator.Address)
	_, v2 := cs1.Validators.GetByAddress(addr2)
	v1.Accum, v1.VotingPower = v2.Accum, v2.VotingPower
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}
}

func addVoteToFromMany(to *ConsensusState, votes []*types.Vote, froms ...*validatorStub) {
	if len(votes) != len(froms) {
		panic("len(votes) and len(froms) must match")
	}
	for i, from := range froms {
		addVoteToFrom(to, from, votes[i])
	}
}

func addVoteToFrom(to *ConsensusState, from *validatorStub, vote *types.Vote) {
	valIndex, _ := to.Validators.GetByAddress(from.PrivValidator.Address)

	to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{valIndex, vote}}
	// added, err := to.TryAddVote(valIndex, vote, "")
	/*
		if _, ok := err.(*types.ErrVoteConflictingSignature); ok {
			// let it fly
		} else if !added {
			fmt.Println("to, from, vote:", to.Height, from.Height, vote.Height)
			panic(fmt.Sprintln("Failed to add vote. Err:", err))
		} else if err != nil {
			panic(fmt.Sprintln("Failed to add vote:", err))
		}*/
}

func signVoteMany(voteType byte, hash []byte, header types.PartSetHeader, vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(vs, voteType, hash, header)
	}
	return votes
}

// add vote to one cs from another
func signAddVoteToFromMany(voteType byte, to *ConsensusState, hash []byte, header types.PartSetHeader, froms ...*validatorStub) {
	for _, from := range froms {
		vote := signVote(from, voteType, hash, header)
		addVoteToFrom(to, from, vote)
	}
}

func signAddVoteToFrom(voteType byte, to *ConsensusState, from *validatorStub, hash []byte, header types.PartSetHeader) *types.Vote {
	vote := signVote(from, voteType, hash, header)
	addVoteToFrom(to, from, vote)
	return vote
}

func ensureNoNewStep(stepCh chan interface{}) {
	timeout := time.NewTicker(ensureTimeout * time.Second)
	select {
	case <-timeout.C:
		break
	case <-stepCh:
		panic("We should be stuck waiting for more votes, not moving to the next step")
	}
}

/*
func ensureNoNewStep(t *testing.T, cs *ConsensusState) {
	timeout := time.NewTicker(ensureTimeout * time.Second)
	select {
	case <-timeout.C:
		break
	case <-cs.NewStepCh():
		panic("We should be stuck waiting for more votes, not moving to the next step")
	}
}

func ensureNewStep(t *testing.T, cs *ConsensusState) *RoundState {
	timeout := time.NewTicker(ensureTimeout * time.Second)
	select {
	case <-timeout.C:
		panic("We should have gone to the next step, not be stuck waiting")
	case rs := <-cs.NewStepCh():
		return rs
	}
}

func waitFor(t *testing.T, cs *ConsensusState, height int, round int, step RoundStepType) {
	for {
		rs := ensureNewStep(t, cs)
		if CompareHRS(rs.Height, rs.Round, rs.Step, height, round, step) < 0 {
			continue
		} else {
			break
		}
	}
}
*/

func incrementHeight(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Height += 1
	}
}

func incrementRound(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Round += 1
	}
}

func validatePrevote(t *testing.T, cs *ConsensusState, round int, privVal *validatorStub, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	var vote *types.Vote
	if vote = prevotes.GetByAddress(privVal.Address); vote == nil {
		panic("Failed to find prevote from validator")
	}
	if blockHash == nil {
		if vote.BlockHash != nil {
			panic(fmt.Sprintf("Expected prevote to be for nil, got %X", vote.BlockHash))
		}
	} else {
		if !bytes.Equal(vote.BlockHash, blockHash) {
			panic(fmt.Sprintf("Expected prevote to be for %X, got %X", blockHash, vote.BlockHash))
		}
	}
}

func validateLastPrecommit(t *testing.T, cs *ConsensusState, privVal *validatorStub, blockHash []byte) {
	votes := cs.LastCommit
	var vote *types.Vote
	if vote = votes.GetByAddress(privVal.Address); vote == nil {
		panic("Failed to find precommit from validator")
	}
	if !bytes.Equal(vote.BlockHash, blockHash) {
		panic(fmt.Sprintf("Expected precommit to be for %X, got %X", blockHash, vote.BlockHash))
	}
}

func validatePrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *validatorStub, votedBlockHash, lockedBlockHash []byte) {
	precommits := cs.Votes.Precommits(thisRound)
	var vote *types.Vote
	if vote = precommits.GetByAddress(privVal.Address); vote == nil {
		panic("Failed to find precommit from validator")
	}

	if votedBlockHash == nil {
		if vote.BlockHash != nil {
			panic("Expected precommit to be for nil")
		}
	} else {
		if !bytes.Equal(vote.BlockHash, votedBlockHash) {
			panic("Expected precommit to be for proposal block")
		}
	}

	if lockedBlockHash == nil {
		if cs.LockedRound != lockRound || cs.LockedBlock != nil {
			panic(fmt.Sprintf("Expected to be locked on nil at round %d. Got locked at round %d with block %v", lockRound, cs.LockedRound, cs.LockedBlock))
		}
	} else {
		if cs.LockedRound != lockRound || !bytes.Equal(cs.LockedBlock.Hash(), lockedBlockHash) {
			panic(fmt.Sprintf("Expected block to be locked on round %d, got %d. Got locked block %X, expected %X", lockRound, cs.LockedRound, cs.LockedBlock.Hash(), lockedBlockHash))
		}
	}

}

func validatePrevoteAndPrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *validatorStub, votedBlockHash, lockedBlockHash []byte) {
	// verify the prevote
	validatePrevote(t, cs, thisRound, privVal, votedBlockHash)
	// verify precommit
	cs.mtx.Lock()
	validatePrecommit(t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
	cs.mtx.Unlock()
}

func fixedConsensusState() *ConsensusState {
	stateDB := dbm.NewMemDB()
	state := sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	return newConsensusState(state, privValidator)

}

func newConsensusState(state *sm.State, pv *types.PrivValidator) *ConsensusState {
	// Get BlockStore
	blockDB := dbm.NewMemDB()
	blockStore := bc.NewBlockStore(blockDB)

	// one for mempool, one for consensus
	mtx, app := new(sync.Mutex), counter.NewCounterApplication(false)
	proxyAppConnMem := tmspcli.NewLocalClient(mtx, app)
	proxyAppConnCon := tmspcli.NewLocalClient(mtx, app)

	// Make Mempool
	mempool := mempl.NewMempool(proxyAppConnMem)

	// Make ConsensusReactor
	cs := NewConsensusState(state, proxyAppConnCon, blockStore, mempool)
	cs.SetPrivValidator(pv)

	evsw := events.NewEventSwitch()
	cs.SetEventSwitch(evsw)
	evsw.Start()
	return cs
}

func randConsensusState(nValidators int) (*ConsensusState, []*validatorStub) {
	// Get State
	state, privVals := randGenesisState(nValidators, false, 10)

	vss := make([]*validatorStub, nValidators)

	cs := newConsensusState(state, privVals[0])

	for i := 0; i < nValidators; i++ {
		vss[i] = NewValidatorStub(privVals[i])
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss
}

func subscribeToVoter(cs *ConsensusState, addr []byte) chan interface{} {
	voteCh0 := subscribeToEvent(cs.evsw, "tester", types.EventStringVote(), 1)
	voteCh := make(chan interface{})
	go func() {
		for {
			v := <-voteCh0
			vote := v.(types.EventDataVote)
			// we only fire for our own votes
			if bytes.Equal(addr, vote.Address) {
				voteCh <- v
			}
		}
	}()
	return voteCh
}

func randGenesisState(numValidators int, randPower bool, minPower int64) (*sm.State, []*types.PrivValidator) {
	db := dbm.NewMemDB()
	genDoc, privValidators := randGenesisDoc(numValidators, randPower, minPower)
	s0 := sm.MakeGenesisState(db, genDoc)
	s0.Save()
	return s0, privValidators
}

func randGenesisDoc(numValidators int, randPower bool, minPower int64) (*types.GenesisDoc, []*types.PrivValidator) {
	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]*types.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = types.GenesisValidator{
			PubKey: val.PubKey,
			Amount: val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))
	return &types.GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     config.GetString("chain_id"),
		Validators:  validators,
	}, privValidators

}

func startTestRound(cs *ConsensusState, height, round int) {
	cs.enterNewRound(height, round)
	cs.startRoutines(0)
}
