package v0

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mempool/mock"
	"github.com/tendermint/tendermint/p2p"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type reactorTestSuite struct {
	reactor *Reactor
	app     proxy.AppConns

	peerID p2p.NodeID

	blockchainChannel   *p2p.Channel
	blockchainInCh      chan p2p.Envelope
	blockchainOutCh     chan p2p.Envelope
	blockchainPeerErrCh chan p2p.PeerError

	peerUpdates *p2p.PeerUpdatesCh
}

func setup(
	t *testing.T,
	genDoc *types.GenesisDoc,
	privVals []types.PrivValidator,
	maxBlockHeight int64,
	chBuf uint,
) *reactorTestSuite {
	t.Helper()

	require.Len(t, privVals, 1, "only one validator can be supported")

	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)

	proxyApp := proxy.NewAppConns(cc)
	require.NoError(t, proxyApp.Start())

	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(blockDB)

	state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
	require.NoError(t, err)

	fastSync := true
	db := dbm.NewMemDB()
	stateStore = sm.NewStore(db)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		mock.Mempool{},
		sm.EmptyEvidencePool{},
	)
	require.NoError(t, stateStore.Save(state))

	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 0, types.BlockID{}, nil)

		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)

			vote, err := types.MakeVote(
				lastBlock.Header.Height,
				lastBlockMeta.BlockID,
				state.Validators,
				privVals[0],
				lastBlock.Header.ChainID,
				time.Now(),
			)
			require.NoError(t, err)

			lastCommit = types.NewCommit(
				vote.Height,
				vote.Round,
				lastBlockMeta.BlockID,
				[]types.CommitSig{vote.CommitSig()},
			)
		}

		thisBlock := makeBlock(blockHeight, state, lastCommit)
		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		state, _, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		require.NoError(t, err)

		blockStore.SaveBlock(thisBlock, thisParts, lastCommit)
	}

	pID := make([]byte, 16)
	_, err = rng.Read(pID)
	require.NoError(t, err)

	rts := &reactorTestSuite{
		app:                 proxyApp,
		blockchainInCh:      make(chan p2p.Envelope, chBuf),
		blockchainOutCh:     make(chan p2p.Envelope, chBuf),
		blockchainPeerErrCh: make(chan p2p.PeerError, chBuf),
		peerUpdates:         p2p.NewPeerUpdates(make(chan p2p.PeerUpdate)),
		peerID:              p2p.NodeID(fmt.Sprintf("%x", pID)),
	}

	rts.blockchainChannel = p2p.NewChannel(
		BlockchainChannel,
		new(bcproto.Message),
		rts.blockchainInCh,
		rts.blockchainOutCh,
		rts.blockchainPeerErrCh,
	)

	rts.reactor = NewReactor(
		log.TestingLogger().With("module", "blockchain"),
		state.Copy(),
		blockExec,
		blockStore,
		nil,
		rts.blockchainChannel,
		rts.peerUpdates,
		fastSync,
	)

	require.NoError(t, rts.reactor.Start())
	require.True(t, rts.reactor.IsRunning())

	t.Cleanup(func() {
		require.NoError(t, rts.reactor.Stop())
		require.NoError(t, rts.app.Stop())
		require.False(t, rts.reactor.IsRunning())
	})

	return rts
}

func randGenesisDoc(
	config *cfg.Config,
	numValidators int,
	randPower bool,
	minPower int64,
) (*types.GenesisDoc, []types.PrivValidator) {
	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)

	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = types.GenesisValidator{
			PubKey: val.PubKey,
			Power:  val.VotingPower,
		}

		privValidators[i] = privVal
	}

	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     config.ChainID(),
		Validators:  validators,
	}, privValidators
}

func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeBlock(height int64, state sm.State, lastCommit *types.Commit) *types.Block {
	block, _ := state.MakeBlock(height, makeTxs(height), lastCommit, nil, state.Validators.GetProposer().Address)
	return block
}

type testApp struct {
	abci.BaseApplication
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return abci.ResponseBeginBlock{}
}

func (app *testApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	return abci.ResponseEndBlock{}
}

func (app *testApp) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Events: []abci.Event{}}
}

func (app *testApp) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}

func simulateRouter(primary *reactorTestSuite, suites []*reactorTestSuite) {
	// create a mapping for efficient suite lookup by peer ID
	suitesByPeerID := make(map[p2p.NodeID]*reactorTestSuite)
	for _, suite := range suites {
		suitesByPeerID[suite.peerID] = suite
	}

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelope to the respective peer (suite).
	go func() {
		for envelope := range primary.blockchainOutCh {
			if envelope.Broadcast {
				for _, s := range suites {
					// broadcast to everyone except source
					if s.peerID != primary.peerID {
						s.blockchainInCh <- p2p.Envelope{
							From:    primary.peerID,
							To:      s.peerID,
							Message: envelope.Message,
						}
					}
				}
			} else {
				other := suitesByPeerID[envelope.To]

				other.blockchainInCh <- p2p.Envelope{
					From:    primary.peerID,
					To:      envelope.To,
					Message: envelope.Message,
				}
			}
		}
	}()

	go func() {
		for pErr := range primary.blockchainPeerErrCh {
			primary.reactor.Logger.Debug("dropped peer error", "err", pErr.Err)
		}
	}()
}

func TestNoBlockResponse(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	genDoc, privVals := randGenesisDoc(config, 1, false, 30)
	maxBlockHeight := int64(65)
	testSuites := []*reactorTestSuite{
		setup(t, genDoc, privVals, maxBlockHeight, 0),
		setup(t, genDoc, privVals, 0, 0),
	}

	require.Equal(t, maxBlockHeight, testSuites[0].reactor.store.Height())

	simulateRouter(testSuites[0], testSuites)
	simulateRouter(testSuites[1], testSuites)

	testCases := []struct {
		height   int64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{100, false},
	}

	secondaryPool := testSuites[1].reactor.pool

	for {
		if secondaryPool.MaxPeerHeight() > 0 && secondaryPool.IsCaughtUp() {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	for _, tc := range testCases {
		block := testSuites[1].reactor.store.LoadBlock(tc.height)
		if tc.existent {
			require.True(t, block != nil)
		} else {
			require.True(t, block == nil, block)
		}
	}
}

// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------

// // NOTE: This is too hard to test without
// // an easy way to add test peer to switch
// // or without significant refactoring of the module.
// // Alternatively we could actually dial a TCP conn but
// // that seems extreme.
// func TestBadBlockStopsPeer(t *testing.T) {
// 	config = cfg.ResetTestRoot("blockchain_reactor_test")
// 	defer os.RemoveAll(config.RootDir)
// 	genDoc, privVals := randGenesisDoc(1, false, 30)

// 	maxBlockHeight := int64(148)

// 	// Other chain needs a different validator set
// 	otherGenDoc, otherPrivVals := randGenesisDoc(1, false, 30)
// 	otherChain := newBlockchainReactor(log.TestingLogger(), otherGenDoc, otherPrivVals, maxBlockHeight)

// 	defer func() {
// 		err := otherChain.reactor.Stop()
// 		require.Error(t, err)
// 		err = otherChain.app.Stop()
// 		require.NoError(t, err)
// 	}()

// 	reactorPairs := make([]BlockchainReactorPair, 4)

// 	reactorPairs[0] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, maxBlockHeight)
// 	reactorPairs[1] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)
// 	reactorPairs[2] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)
// 	reactorPairs[3] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)

// 	switches := p2p.MakeConnectedSwitches(config.P2P, 4, func(i int, s *p2p.Switch) *p2p.Switch {
// 		s.AddReactor("BLOCKCHAIN", reactorPairs[i].reactor)
// 		return s

// 	}, p2p.Connect2Switches)

// 	defer func() {
// 		for _, r := range reactorPairs {
// 			err := r.reactor.Stop()
// 			require.NoError(t, err)

// 			err = r.app.Stop()
// 			require.NoError(t, err)
// 		}
// 	}()

// 	for {
// 		time.Sleep(1 * time.Second)
// 		caughtUp := true
// 		for _, r := range reactorPairs {
// 			if !r.reactor.pool.IsCaughtUp() {
// 				caughtUp = false
// 			}
// 		}
// 		if caughtUp {
// 			break
// 		}
// 	}

// 	// at this time, reactors[0-3] is the newest
// 	assert.Equal(t, 3, reactorPairs[1].reactor.Switch.Peers().Size())

// 	// Mark reactorPairs[3] as an invalid peer. Fiddling with .store without a mutex is a data
// 	// race, but can't be easily avoided.
// 	reactorPairs[3].reactor.store = otherChain.reactor.store

// 	lastReactorPair := newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)
// 	reactorPairs = append(reactorPairs, lastReactorPair)

// 	switches = append(switches, p2p.MakeConnectedSwitches(config.P2P, 1, func(i int, s *p2p.Switch) *p2p.Switch {
// 		s.AddReactor("BLOCKCHAIN", reactorPairs[len(reactorPairs)-1].reactor)
// 		return s

// 	}, p2p.Connect2Switches)...)

// 	for i := 0; i < len(reactorPairs)-1; i++ {
// 		p2p.Connect2Switches(switches, i, len(reactorPairs)-1)
// 	}

// 	for {
// 		if lastReactorPair.reactor.pool.IsCaughtUp() || lastReactorPair.reactor.Switch.Peers().Size() == 0 {
// 			break
// 		}

// 		time.Sleep(1 * time.Second)
// 	}

// 	assert.True(t, lastReactorPair.reactor.Switch.Peers().Size() < len(reactorPairs)-1)
// }
