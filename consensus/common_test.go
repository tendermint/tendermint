package consensus

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-events"
	bc "github.com/tendermint/tendermint/blockchain"
	mempl "github.com/tendermint/tendermint/mempool"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmspcli "github.com/tendermint/tmsp/client"
	tmsp "github.com/tendermint/tmsp/types"

	"github.com/tendermint/tmsp/example/counter"
)

var config cfg.Config // NOTE: must be reset for each _test.go file
var ensureTimeout = time.Duration(2)

type validatorStub struct {
	Index  int // Validator index. NOTE: we don't assume validator set changes.
	Height int
	Round  int
	*types.PrivValidator
}

func NewValidatorStub(privValidator *types.PrivValidator, valIndex int) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
	}
}

func (vs *validatorStub) signVote(voteType byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	vote := &types.Vote{
		ValidatorIndex:   vs.Index,
		ValidatorAddress: vs.PrivValidator.Address,
		Height:           vs.Height,
		Round:            vs.Round,
		Type:             voteType,
		BlockHash:        hash,
		BlockPartsHeader: header,
	}
	err := vs.PrivValidator.SignVote(config.GetString("chain_id"), vote)
	return vote, err
}

//-------------------------------------------------------------------------------
// Convenience functions

// Sign vote for type/hash/header
func signVote(vs *validatorStub, voteType byte, hash []byte, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

// Create proposal block from cs1 but sign it with vs
func decideProposal(cs1 *ConsensusState, vs *validatorStub, height, round int) (proposal *types.Proposal, block *types.Block) {
	block, blockParts := cs1.createProposalBlock()
	if block == nil { // on error
		panic("error creating proposal block")
	}

	// Make proposal
	proposal = types.NewProposal(height, round, blockParts.Header(), cs1.Votes.POLRound())
	if err := vs.SignProposal(config.GetString("chain_id"), proposal); err != nil {
		panic(err)
	}
	return
}

func addVotes(to *ConsensusState, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signVotes(voteType byte, hash []byte, header types.PartSetHeader, vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(vs, voteType, hash, header)
	}
	return votes
}

func signAddVotes(to *ConsensusState, voteType byte, hash []byte, header types.PartSetHeader, vss ...*validatorStub) {
	votes := signVotes(voteType, hash, header, vss...)
	addVotes(to, votes...)
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
	return newConsensusState(state, privValidator, counter.NewCounterApplication(true))

}

func newConsensusState(state *sm.State, pv *types.PrivValidator, app tmsp.Application) *ConsensusState {
	// Get BlockStore
	blockDB := dbm.NewMemDB()
	blockStore := bc.NewBlockStore(blockDB)

	// one for mempool, one for consensus
	mtx := new(sync.Mutex)
	proxyAppConnMem := tmspcli.NewLocalClient(mtx, app)
	proxyAppConnCon := tmspcli.NewLocalClient(mtx, app)

	// Make Mempool
	mempool := mempl.NewMempool(config, proxyAppConnMem)

	// Make ConsensusReactor
	cs := NewConsensusState(config, state, proxyAppConnCon, blockStore, mempool)
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

	cs := newConsensusState(state, privVals[0], counter.NewCounterApplication(true))

	for i := 0; i < nValidators; i++ {
		vss[i] = NewValidatorStub(privVals[i], i)
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
			if bytes.Equal(addr, vote.Vote.ValidatorAddress) {
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
