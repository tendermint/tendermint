package v1

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mempool/mock"
	"github.com/tendermint/tendermint/p2p"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var config *cfg.Config

func randGenesisDoc(numValidators int) (*types.GenesisDoc, []types.PrivValidator) {
	validators, privValidators, thresholdPublicKey := types.GenerateGenesisValidators(numValidators)
	return &types.GenesisDoc{
		GenesisTime:        tmtime.Now(),
		ChainID:            config.ChainID(),
		Validators:         validators,
		ThresholdPublicKey: thresholdPublicKey,
		QuorumHash:         crypto.RandQuorumHash(),
	}, privValidators
}

func makeVote(
	t *testing.T,
	header *types.Header,
	blockID types.BlockID,
	stateID types.StateID,
	valset *types.ValidatorSet,
	privVal types.PrivValidator) *types.Vote {

	proTxHash, err := privVal.GetProTxHash()
	require.NoError(t, err)

	valIdx, _ := valset.GetByProTxHash(proTxHash)
	vote := &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     valIdx,
		Height:             header.Height,
		Round:              1,
		Type:               tmproto.PrecommitType,
		BlockID:            blockID,
		StateID:            stateID,
	}

	vpb := vote.ToProto()

	_ = privVal.SignVote(header.ChainID, valset.QuorumType, valset.QuorumHash, vpb)
	vote.BlockSignature = vpb.BlockSignature
	vote.StateSignature = vpb.StateSignature

	return vote
}

type BlockchainReactorPair struct {
	bcR  *BlockchainReactor
	conR *consensusReactorTest
}

func newBlockchainReactor(
	t *testing.T,
	logger log.Logger,
	genDoc *types.GenesisDoc,
	privVals []types.PrivValidator,
	maxBlockHeight int64) *BlockchainReactor {
	if len(privVals) != 1 {
		panic("only support one validator")
	}

	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	if err != nil {
		panic(fmt.Errorf("error start app: %w", err))
	}

	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(blockDB)

	state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
	if err != nil {
		panic(fmt.Errorf("error constructing state from genesis file: %w", err))
	}

	// Make the BlockchainReactor itself.
	// NOTE we have to create and commit the blocks first because
	// pool.height is determined from the store.
	fastSync := true
	db := dbm.NewMemDB()
	stateStore = sm.NewStore(db)
	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(), proxyApp.Query(),
		mock.Mempool{}, sm.EmptyEvidencePool{}, nil)
	if err = stateStore.Save(state); err != nil {
		panic(err)
	}

	// let's add some blocks in
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 1, types.BlockID{}, types.StateID{}, nil, nil, nil, nil)
		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)

			vote := makeVote(t, &lastBlock.Header, lastBlockMeta.BlockID, lastBlockMeta.StateID, state.Validators, privVals[0])
			// since there is only 1 vote, use it as threshold
			commitSig := vote.CommitSig()
			lastCommit = types.NewCommit(vote.Height, vote.Round, lastBlockMeta.BlockID, lastBlockMeta.StateID,
				[]types.CommitSig{commitSig}, nil, commitSig.BlockSignature, commitSig.StateSignature)
		}

		thisBlock := makeBlock(blockHeight, nil, state, lastCommit)

		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		state, _, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		if err != nil {
			panic(fmt.Errorf("error apply block: %w", err))
		}

		blockStore.SaveBlock(thisBlock, thisParts, lastCommit)
	}

	bcReactor := NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	return bcReactor
}

func newBlockchainReactorPair(
	t *testing.T,
	logger log.Logger,
	genDoc *types.GenesisDoc,
	privVals []types.PrivValidator,
	maxBlockHeight int64) BlockchainReactorPair {

	consensusReactor := &consensusReactorTest{}
	consensusReactor.BaseReactor = *p2p.NewBaseReactor("Consensus reactor", consensusReactor)

	return BlockchainReactorPair{
		newBlockchainReactor(t, logger, genDoc, privVals, maxBlockHeight),
		consensusReactor}
}

type consensusReactorTest struct {
	p2p.BaseReactor     // BaseService + p2p.Switch
	switchedToConsensus bool
	mtx                 sync.Mutex
}

func (conR *consensusReactorTest) SwitchToConsensus(state sm.State, blocksSynced bool) {
	conR.mtx.Lock()
	defer conR.mtx.Unlock()
	conR.switchedToConsensus = true
}

func TestFastSyncNoBlockResponse(t *testing.T) {

	config = cfg.ResetTestRoot("blockchain_new_reactor_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1)

	maxBlockHeight := int64(65)

	reactorPairs := make([]BlockchainReactorPair, 2)

	logger := log.TestingLogger()
	reactorPairs[0] = newBlockchainReactorPair(t, logger, genDoc, privVals, maxBlockHeight)
	reactorPairs[1] = newBlockchainReactorPair(t, logger, genDoc, privVals, 0)

	p2p.MakeConnectedSwitches(config.P2P, 2, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("BLOCKCHAIN", reactorPairs[i].bcR)
		s.AddReactor("CONSENSUS", reactorPairs[i].conR)
		moduleName := fmt.Sprintf("blockchain-%v", i)
		reactorPairs[i].bcR.SetLogger(logger.With("module", moduleName))

		return s

	}, p2p.Connect2Switches)

	defer func() {
		for _, r := range reactorPairs {
			_ = r.bcR.Stop()
			_ = r.conR.Stop()
		}
	}()

	tests := []struct {
		height   int64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{maxBlockHeight + 100, false},
	}

	for {
		time.Sleep(10 * time.Millisecond)
		reactorPairs[1].conR.mtx.Lock()
		if reactorPairs[1].conR.switchedToConsensus {
			reactorPairs[1].conR.mtx.Unlock()
			break
		}
		reactorPairs[1].conR.mtx.Unlock()
	}

	assert.Equal(t, maxBlockHeight, reactorPairs[0].bcR.store.Height())

	for _, tt := range tests {
		block := reactorPairs[1].bcR.store.LoadBlock(tt.height)
		if tt.existent {
			assert.True(t, block != nil)
		} else {
			assert.True(t, block == nil)
		}
	}
}

// NOTE: This is too hard to test without
// an easy way to add test peer to switch
// or without significant refactoring of the module.
// Alternatively we could actually dial a TCP conn but
// that seems extreme.
func TestFastSyncBadBlockStopsPeer(t *testing.T) {
	numNodes := 4
	maxBlockHeight := int64(148)

	config = cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1)

	otherChain := newBlockchainReactorPair(t, log.TestingLogger(), genDoc, privVals, maxBlockHeight)
	defer func() {
		_ = otherChain.bcR.Stop()
		_ = otherChain.conR.Stop()
	}()

	reactorPairs := make([]BlockchainReactorPair, numNodes)
	logger := make([]log.Logger, numNodes)

	for i := 0; i < numNodes; i++ {
		logger[i] = log.TestingLogger()
		height := int64(0)
		if i == 0 {
			height = maxBlockHeight
		}
		reactorPairs[i] = newBlockchainReactorPair(t, logger[i], genDoc, privVals, height)
	}

	switches := p2p.MakeConnectedSwitches(config.P2P, numNodes, func(i int, s *p2p.Switch) *p2p.Switch {
		reactorPairs[i].conR.mtx.Lock()
		s.AddReactor("BLOCKCHAIN", reactorPairs[i].bcR)
		s.AddReactor("CONSENSUS", reactorPairs[i].conR)
		moduleName := fmt.Sprintf("blockchain-%v", i)
		reactorPairs[i].bcR.SetLogger(logger[i].With("module", moduleName))
		reactorPairs[i].conR.mtx.Unlock()
		return s

	}, p2p.Connect2Switches)

	defer func() {
		for _, r := range reactorPairs {
			_ = r.bcR.Stop()
			_ = r.conR.Stop()
		}
	}()

outerFor:
	for {
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < numNodes; i++ {
			reactorPairs[i].conR.mtx.Lock()
			if !reactorPairs[i].conR.switchedToConsensus {
				reactorPairs[i].conR.mtx.Unlock()
				continue outerFor
			}
			reactorPairs[i].conR.mtx.Unlock()
		}
		break
	}

	// at this time, reactors[0-3] is the newest
	assert.Equal(t, numNodes-1, reactorPairs[1].bcR.Switch.Peers().Size())

	// mark last reactorPair as an invalid peer
	reactorPairs[numNodes-1].bcR.store = otherChain.bcR.store

	lastLogger := log.TestingLogger()
	lastReactorPair := newBlockchainReactorPair(t, lastLogger, genDoc, privVals, 0)
	reactorPairs = append(reactorPairs, lastReactorPair)

	switches = append(switches, p2p.MakeConnectedSwitches(config.P2P, 1, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("BLOCKCHAIN", reactorPairs[len(reactorPairs)-1].bcR)
		s.AddReactor("CONSENSUS", reactorPairs[len(reactorPairs)-1].conR)
		moduleName := fmt.Sprintf("blockchain-%v", len(reactorPairs)-1)
		reactorPairs[len(reactorPairs)-1].bcR.SetLogger(lastLogger.With("module", moduleName))
		return s

	}, p2p.Connect2Switches)...)

	for i := 0; i < len(reactorPairs)-1; i++ {
		p2p.Connect2Switches(switches, i, len(reactorPairs)-1)
	}

	for {
		time.Sleep(1 * time.Second)
		lastReactorPair.conR.mtx.Lock()
		if lastReactorPair.conR.switchedToConsensus {
			lastReactorPair.conR.mtx.Unlock()
			break
		}
		lastReactorPair.conR.mtx.Unlock()

		if lastReactorPair.bcR.Switch.Peers().Size() == 0 {
			break
		}
	}

	assert.True(t, lastReactorPair.bcR.Switch.Peers().Size() < len(reactorPairs)-1)
}

//----------------------------------------------
// utility funcs

func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeBlock(height int64, coreChainLock *types.CoreChainLock, state sm.State,
	lastCommit *types.Commit) *types.Block {
	block, _ := state.MakeBlock(height, coreChainLock, makeTxs(height), lastCommit,
		nil, state.Validators.GetProposer().ProTxHash)
	return block
}

type testApp struct {
	abci.BaseApplication
}
