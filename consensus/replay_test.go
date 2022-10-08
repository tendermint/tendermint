package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/abci/types/mocks"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/test"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/mempool"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func TestMain(m *testing.M) {
	config = ResetConfig("consensus_reactor_test")
	consensusReplayConfig = ResetConfig("consensus_replay_test")
	configStateTest := ResetConfig("consensus_state_test")
	configMempoolTest := ResetConfig("consensus_mempool_test")
	configByzantineTest := ResetConfig("consensus_byzantine_test")
	code := m.Run()
	os.RemoveAll(config.RootDir)
	os.RemoveAll(consensusReplayConfig.RootDir)
	os.RemoveAll(configStateTest.RootDir)
	os.RemoveAll(configMempoolTest.RootDir)
	os.RemoveAll(configByzantineTest.RootDir)
	os.Exit(code)
}

// These tests ensure we can always recover from failure at any part of the consensus process.
// There are two general failure scenarios: failure during consensus, and failure while applying the block.
// Only the latter interacts with the app and store,
// but the former has to deal with restrictions on re-use of priv_validator keys.
// The `WAL Tests` are for failures during the consensus;
// the `Handshake Tests` are for failures in applying the block.
// With the help of the WAL, we can recover from it all!

//------------------------------------------------------------------------------------------
// WAL Tests

// TODO: It would be better to verify explicitly which states we can recover from without the wal
// and which ones we need the wal for - then we'd also be able to only flush the
// wal writer when we need to, instead of with every message.

func startNewStateAndWaitForBlock(t *testing.T, consensusReplayConfig *cfg.Config,
	lastBlockHeight int64, blockDB dbm.DB, stateStore sm.Store) {
	logger := log.TestingLogger()
	state, _ := stateStore.LoadFromDBOrGenesisFile(consensusReplayConfig.GenesisFile())
	privValidator := loadPrivValidator(consensusReplayConfig)
	cs := newStateWithConfigAndBlockStore(
		consensusReplayConfig,
		state,
		privValidator,
		kvstore.NewInMemoryApplication(),
		blockDB,
	)
	cs.SetLogger(logger)

	bytes, _ := os.ReadFile(cs.config.WalFile())
	t.Logf("====== WAL: \n\r%X\n", bytes)

	err := cs.Start()
	require.NoError(t, err)
	defer func() {
		if err := cs.Stop(); err != nil {
			t.Error(err)
		}
	}()

	// This is just a signal that we haven't halted; its not something contained
	// in the WAL itself. Assuming the consensus state is running, replay of any
	// WAL, including the empty one, should eventually be followed by a new
	// block, or else something is wrong.
	newBlockSub, err := cs.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
	require.NoError(t, err)
	select {
	case <-newBlockSub.Out():
	case <-newBlockSub.Cancelled():
		t.Fatal("newBlockSub was canceled")
	case <-time.After(120 * time.Second):
		t.Fatal("Timed out waiting for new block (see trace above)")
	}
}

func sendTxs(ctx context.Context, cs *State) {
	for i := 0; i < 256; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			tx := kvstore.NewTxFromID(i)
			if err := assertMempool(cs.txNotifier).CheckTx(tx, func(resp *abci.ResponseCheckTx) {
				if resp.Code != 0 {
					panic(fmt.Sprintf("Unexpected code: %d, log: %s", resp.Code, resp.Log))
				}
			}, mempl.TxInfo{}); err != nil {
				panic(err)
			}
			i++
		}
	}
}

// TestWALCrash uses crashing WAL to test we can recover from any WAL failure.
func TestWALCrash(t *testing.T) {
	testCases := []struct {
		name         string
		initFn       func(dbm.DB, *State, context.Context)
		heightToStop int64
	}{
		{"empty block",
			func(stateDB dbm.DB, cs *State, ctx context.Context) {},
			1},
		{"many non-empty blocks",
			func(stateDB dbm.DB, cs *State, ctx context.Context) {
				go sendTxs(ctx, cs)
			},
			3},
	}

	for i, tc := range testCases {
		tc := tc
		consensusReplayConfig := ResetConfig(fmt.Sprintf("%s_%d", t.Name(), i))
		t.Run(tc.name, func(t *testing.T) {
			crashWALandCheckLiveness(t, consensusReplayConfig, tc.initFn, tc.heightToStop)
		})
	}
}

func crashWALandCheckLiveness(t *testing.T, consensusReplayConfig *cfg.Config,
	initFn func(dbm.DB, *State, context.Context), heightToStop int64) {
	walPanicked := make(chan error)
	crashingWal := &crashingWAL{panicCh: walPanicked, heightToStop: heightToStop}

	i := 1
LOOP:
	for {
		t.Logf("====== LOOP %d\n", i)

		// create consensus state from a clean slate
		logger := log.NewNopLogger()
		blockDB := dbm.NewMemDB()
		stateDB := blockDB
		stateStore := sm.NewStore(stateDB, sm.StoreOptions{
			DiscardFinalizeBlockResponses: false,
		})
		state, err := sm.MakeGenesisStateFromFile(consensusReplayConfig.GenesisFile())
		require.NoError(t, err)
		privValidator := loadPrivValidator(consensusReplayConfig)
		cs := newStateWithConfigAndBlockStore(
			consensusReplayConfig,
			state,
			privValidator,
			kvstore.NewInMemoryApplication(),
			blockDB,
		)
		cs.SetLogger(logger)

		// start sending transactions
		ctx, cancel := context.WithCancel(context.Background())
		initFn(stateDB, cs, ctx)

		// clean up WAL file from the previous iteration
		walFile := cs.config.WalFile()
		os.Remove(walFile)

		// set crashing WAL
		csWal, err := cs.OpenWAL(walFile)
		require.NoError(t, err)
		crashingWal.next = csWal

		// reset the message counter
		crashingWal.msgIndex = 1
		cs.wal = crashingWal

		// start consensus state
		err = cs.Start()
		require.NoError(t, err)

		i++

		select {
		case err := <-walPanicked:
			t.Logf("WAL panicked: %v", err)

			// make sure we can make blocks after a crash
			startNewStateAndWaitForBlock(t, consensusReplayConfig, cs.Height, blockDB, stateStore)

			// stop consensus state and transactions sender (initFn)
			cs.Stop() //nolint:errcheck // Logging this error causes failure
			cancel()

			// if we reached the required height, exit
			if _, ok := err.(ReachedHeightToStopError); ok {
				break LOOP
			}
		case <-time.After(10 * time.Second):
			t.Fatal("WAL did not panic for 10 seconds (check the log)")
		}
	}
}

// crashingWAL is a WAL which crashes or rather simulates a crash during Save
// (before and after). It remembers a message for which we last panicked
// (lastPanickedForMsgIndex), so we don't panic for it in subsequent iterations.
type crashingWAL struct {
	next         WAL
	panicCh      chan error
	heightToStop int64

	msgIndex                int // current message index
	lastPanickedForMsgIndex int // last message for which we panicked
}

var _ WAL = &crashingWAL{}

// WALWriteError indicates a WAL crash.
type WALWriteError struct {
	msg string
}

func (e WALWriteError) Error() string {
	return e.msg
}

// ReachedHeightToStopError indicates we've reached the required consensus
// height and may exit.
type ReachedHeightToStopError struct {
	height int64
}

func (e ReachedHeightToStopError) Error() string {
	return fmt.Sprintf("reached height to stop %d", e.height)
}

// Write simulate WAL's crashing by sending an error to the panicCh and then
// exiting the cs.receiveRoutine.
func (w *crashingWAL) Write(m WALMessage) error {
	if endMsg, ok := m.(EndHeightMessage); ok {
		if endMsg.Height == w.heightToStop {
			w.panicCh <- ReachedHeightToStopError{endMsg.Height}
			runtime.Goexit()
			return nil
		}

		return w.next.Write(m)
	}

	if w.msgIndex > w.lastPanickedForMsgIndex {
		w.lastPanickedForMsgIndex = w.msgIndex
		_, file, line, _ := runtime.Caller(1)
		w.panicCh <- WALWriteError{fmt.Sprintf("failed to write %T to WAL (fileline: %s:%d)", m, file, line)}
		runtime.Goexit()
		return nil
	}

	w.msgIndex++
	return w.next.Write(m)
}

func (w *crashingWAL) WriteSync(m WALMessage) error {
	return w.Write(m)
}

func (w *crashingWAL) FlushAndSync() error { return w.next.FlushAndSync() }

func (w *crashingWAL) SearchForEndHeight(
	height int64,
	options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	return w.next.SearchForEndHeight(height, options)
}

func (w *crashingWAL) Start() error { return w.next.Start() }
func (w *crashingWAL) Stop() error  { return w.next.Stop() }
func (w *crashingWAL) Wait()        { w.next.Wait() }

// ------------------------------------------------------------------------------------------

const numBlocks = 6

//---------------------------------------
// Test handshake/replay

// 0 - all synced up
// 1 - saved block but app and state are behind
// 2 - save block and committed but state is behind
// 3 - save block and committed with truncated block store and state behind
var modes = []uint{0, 1, 2, 3}

// Sync from scratch
func TestHandshakeReplayAll(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, 0, m)
	}
}

// Sync many, not from scratch
func TestHandshakeReplaySome(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, 2, m)
	}
}

// Sync from lagging by one
func TestHandshakeReplayOne(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, numBlocks-1, m)
	}
}

// Sync from caught up
func TestHandshakeReplayNone(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, numBlocks, m)
	}
}

func tempWALWithData(data []byte) string {
	walFile, err := os.CreateTemp("", "wal")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp WAL file: %v", err))
	}
	_, err = walFile.Write(data)
	if err != nil {
		panic(fmt.Sprintf("failed to write to temp WAL file: %v", err))
	}
	if err := walFile.Close(); err != nil {
		panic(fmt.Sprintf("failed to close temp WAL file: %v", err))
	}
	return walFile.Name()
}

// Make some blocks. Start a fresh app and apply nBlocks blocks.
// Then restart the app and sync it up with the remaining blocks
func testHandshakeReplay(t *testing.T, config *cfg.Config, nBlocks int, mode uint) {
	var (
		chain        []*types.Block
		commits      []*types.Commit
		store        *mockBlockStore
		stateDB      dbm.DB
		genesisState sm.State
		mempool      = emptyMempool{}
		evpool       = sm.EmptyEvidencePool{}
	)

	testConfig := ResetConfig(fmt.Sprintf("%d_%d_s", nBlocks, mode))
	t.Cleanup(func() {
		_ = os.RemoveAll(testConfig.RootDir)
	})
	walBody, err := WALWithNBlocks(t, numBlocks)
	require.NoError(t, err)
	walFile := tempWALWithData(walBody)
	config.Consensus.SetWalFile(walFile)

	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())

	wal, err := NewWAL(walFile)
	require.NoError(t, err)
	wal.SetLogger(log.TestingLogger())
	err = wal.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := wal.Stop(); err != nil {
			t.Error(err)
		}
	})
	chain, commits, err = makeBlockchainFromWAL(wal)
	require.NoError(t, err)
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	stateDB, genesisState, store = stateAndStore(t, config, pubKey, kvstore.AppVersion)

	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardFinalizeBlockResponses: false,
	})
	t.Cleanup(func() {
		_ = stateStore.Close()
	})
	store.chain = chain
	store.commits = commits

	state := genesisState.Copy()
	// run the chain through state.ApplyBlock to build up the tendermint state
	state = buildTMStateFromChain(t, config, stateStore, mempool, evpool, state, chain, nBlocks, mode)
	latestAppHash := state.AppHash

	// make a new client creator
	kvstoreApp := kvstore.NewPersistentApplication(
		filepath.Join(config.DBDir(), fmt.Sprintf("replay_test_%d_%d_a", nBlocks, mode)))
	t.Cleanup(func() {
		_ = kvstoreApp.Close()
	})

	clientCreator2 := proxy.NewLocalClientCreator(kvstoreApp)
	if nBlocks > 0 {
		// run nBlocks against a new client to build up the app state.
		// use a throwaway tendermint state
		proxyApp := proxy.NewAppConns(clientCreator2, proxy.NopMetrics())
		stateDB1 := dbm.NewMemDB()
		stateStore := sm.NewStore(stateDB1, sm.StoreOptions{
			DiscardFinalizeBlockResponses: false,
		})
		err := stateStore.Save(genesisState)
		require.NoError(t, err)
		buildAppStateFromChain(t, proxyApp, stateStore, mempool, evpool, genesisState, chain, nBlocks, mode)
	}

	// Prune block store if requested
	expectError := false
	if mode == 3 {
		pruned, err := store.PruneBlocks(2)
		require.NoError(t, err)
		require.EqualValues(t, 1, pruned)
		expectError = int64(nBlocks) < 2
	}

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	handshaker := NewHandshaker(stateStore, state, store, genDoc)
	proxyApp := proxy.NewAppConns(clientCreator2, proxy.NopMetrics())
	if err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}

	t.Cleanup(func() {
		if err := proxyApp.Stop(); err != nil {
			t.Error(err)
		}
	})

	err = handshaker.Handshake(proxyApp)
	if expectError {
		require.Error(t, err)
		return
	} else if err != nil {
		t.Fatalf("Error on abci handshake: %v", err)
	}

	// get the latest app hash from the app
	res, err := proxyApp.Query().Info(context.Background(), &abci.RequestInfo{Version: ""})
	if err != nil {
		t.Fatal(err)
	}

	// the app hash should be synced up
	if !bytes.Equal(latestAppHash, res.LastBlockAppHash) {
		t.Fatalf(
			"Expected app hashes to match after handshake/replay. got %X, expected %X",
			res.LastBlockAppHash,
			latestAppHash)
	}

	expectedBlocksToSync := numBlocks - nBlocks
	if nBlocks == numBlocks && mode > 0 {
		expectedBlocksToSync++
	} else if nBlocks > 0 && mode == 1 {
		expectedBlocksToSync++
	}

	if handshaker.NBlocks() != expectedBlocksToSync {
		t.Fatalf("Expected handshake to sync %d blocks, got %d", expectedBlocksToSync, handshaker.NBlocks())
	}
}

func applyBlock(t *testing.T, stateStore sm.Store, mempool mempool.Mempool, evpool sm.EvidencePool, st sm.State, blk *types.Block, proxyApp proxy.AppConns) sm.State {
	testPartSize := types.BlockPartSizeBytes
	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(), mempool, evpool)

	bps, err := blk.MakePartSet(testPartSize)
	require.NoError(t, err)
	blkID := types.BlockID{Hash: blk.Hash(), PartSetHeader: bps.Header()}
	newState, _, err := blockExec.ApplyBlock(st, blkID, blk)
	require.NoError(t, err)
	return newState
}

func buildAppStateFromChain(t *testing.T, proxyApp proxy.AppConns, stateStore sm.Store, mempool mempool.Mempool, evpool sm.EvidencePool,
	state sm.State, chain []*types.Block, nBlocks int, mode uint) {
	// start a new app without handshake, play nBlocks blocks
	if err := proxyApp.Start(); err != nil {
		panic(err)
	}
	defer proxyApp.Stop() //nolint:errcheck // ignore

	state.Version.Consensus.App = kvstore.AppVersion // simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	if _, err := proxyApp.Consensus().InitChain(context.Background(), &abci.RequestInitChain{
		Validators: validators,
	}); err != nil {
		panic(err)
	}
	if err := stateStore.Save(state); err != nil { // save height 1's validatorsInfo
		panic(err)
	}
	switch mode {
	case 0:
		for i := 0; i < nBlocks; i++ {
			block := chain[i]
			state = applyBlock(t, stateStore, mempool, evpool, state, block, proxyApp)
		}
	case 1, 2, 3:
		for i := 0; i < nBlocks-1; i++ {
			block := chain[i]
			state = applyBlock(t, stateStore, mempool, evpool, state, block, proxyApp)
		}

		if mode == 2 || mode == 3 {
			// update the kvstore height and apphash
			// as if we ran commit but not
			state = applyBlock(t, stateStore, mempool, evpool, state, chain[nBlocks-1], proxyApp)
		}
	default:
		panic(fmt.Sprintf("unknown mode %v", mode))
	}

}

func buildTMStateFromChain(
	t *testing.T,
	config *cfg.Config,
	stateStore sm.Store,
	mempool mempool.Mempool,
	evpool sm.EvidencePool,
	state sm.State,
	chain []*types.Block,
	nBlocks int,
	mode uint) sm.State {
	// run the whole chain against this client to build up the tendermint state
	clientCreator := proxy.NewLocalClientCreator(
		kvstore.NewPersistentApplication(
			filepath.Join(config.DBDir(), fmt.Sprintf("replay_test_%d_%d_t", nBlocks, mode))))
	proxyApp := proxy.NewAppConns(clientCreator, proxy.NopMetrics())
	if err := proxyApp.Start(); err != nil {
		panic(err)
	}
	defer proxyApp.Stop() //nolint:errcheck

	state.Version.Consensus.App = kvstore.AppVersion // simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	if _, err := proxyApp.Consensus().InitChain(context.Background(), &abci.RequestInitChain{
		Validators: validators,
	}); err != nil {
		panic(err)
	}
	if err := stateStore.Save(state); err != nil { // save height 1's validatorsInfo
		panic(err)
	}
	switch mode {
	case 0:
		// sync right up
		for _, block := range chain {
			state = applyBlock(t, stateStore, mempool, evpool, state, block, proxyApp)
		}

	case 1, 2, 3:
		// sync up to the penultimate as if we stored the block.
		// whether we commit or not depends on the appHash
		for _, block := range chain[:len(chain)-1] {
			state = applyBlock(t, stateStore, mempool, evpool, state, block, proxyApp)
		}

		// apply the final block to a state copy so we can
		// get the right next appHash but keep the state back
		applyBlock(t, stateStore, mempool, evpool, state, chain[len(chain)-1], proxyApp)
	default:
		panic(fmt.Sprintf("unknown mode %v", mode))
	}

	return state
}

func TestHandshakePanicsIfAppReturnsWrongAppHash(t *testing.T) {
	// 1. Initialize tendermint and commit 3 blocks with the following app hashes:
	//		- 0x01
	//		- 0x02
	//		- 0x03
	config := ResetConfig("handshake_test_")
	defer os.RemoveAll(config.RootDir)
	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	const appVersion = 0x0
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, config, pubKey, appVersion)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardFinalizeBlockResponses: false,
	})
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	state.LastValidators = state.Validators.Copy()
	// mode = 0 for committing all the blocks
	blocks, err := test.MakeBlocks(3, state, []types.PrivValidator{privVal})
	require.NoError(t, err)

	store.chain = blocks

	// 2. Tendermint must panic if app returns wrong hash for the first block
	//		- RANDOM HASH
	//		- 0x02
	//		- 0x03
	{
		app := &badApp{numBlocks: 3, allHashesAreWrong: true}
		clientCreator := proxy.NewLocalClientCreator(app)
		proxyApp := proxy.NewAppConns(clientCreator, proxy.NopMetrics())
		err := proxyApp.Start()
		require.NoError(t, err)
		t.Cleanup(func() {
			if err := proxyApp.Stop(); err != nil {
				t.Error(err)
			}
		})

		assert.Panics(t, func() {
			h := NewHandshaker(stateStore, state, store, genDoc)
			if err = h.Handshake(proxyApp); err != nil {
				t.Log(err)
			}
		})
	}

	// 3. Tendermint must panic if app returns wrong hash for the last block
	//		- 0x01
	//		- 0x02
	//		- RANDOM HASH
	{
		app := &badApp{numBlocks: 3, onlyLastHashIsWrong: true}
		clientCreator := proxy.NewLocalClientCreator(app)
		proxyApp := proxy.NewAppConns(clientCreator, proxy.NopMetrics())
		err := proxyApp.Start()
		require.NoError(t, err)
		t.Cleanup(func() {
			if err := proxyApp.Stop(); err != nil {
				t.Error(err)
			}
		})

		assert.Panics(t, func() {
			h := NewHandshaker(stateStore, state, store, genDoc)
			if err = h.Handshake(proxyApp); err != nil {
				t.Log(err)
			}
		})
	}
}

type badApp struct {
	abci.BaseApplication
	numBlocks           byte
	height              byte
	allHashesAreWrong   bool
	onlyLastHashIsWrong bool
}

func (app *badApp) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.height++
	if app.onlyLastHashIsWrong {
		if app.height == app.numBlocks {
			return &abci.ResponseFinalizeBlock{AgreedAppData: tmrand.Bytes(8)}, nil
		}
		return &abci.ResponseFinalizeBlock{AgreedAppData: []byte{app.height}}, nil
	} else if app.allHashesAreWrong {
		return &abci.ResponseFinalizeBlock{AgreedAppData: tmrand.Bytes(8)}, nil
	}

	panic("either allHashesAreWrong or onlyLastHashIsWrong must be set")
}

//--------------------------
// utils for making blocks

func makeBlockchainFromWAL(wal WAL) ([]*types.Block, []*types.Commit, error) {
	var height int64

	// Search for height marker
	gr, found, err := wal.SearchForEndHeight(height, &WALSearchOptions{})
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("wal does not contain height %d", height)
	}
	defer gr.Close()

	// log.Notice("Build a blockchain by reading from the WAL")

	var (
		blocks          []*types.Block
		commits         []*types.Commit
		thisBlockParts  *types.PartSet
		thisBlockCommit *types.Commit
	)

	dec := NewWALDecoder(gr)
	for {
		msg, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, nil, err
		}

		piece := readPieceFromWAL(msg)
		if piece == nil {
			continue
		}

		switch p := piece.(type) {
		case EndHeightMessage:
			// if its not the first one, we have a full block
			if thisBlockParts != nil {
				var pbb = new(tmproto.Block)
				bz, err := io.ReadAll(thisBlockParts.GetReader())
				if err != nil {
					panic(err)
				}
				err = proto.Unmarshal(bz, pbb)
				if err != nil {
					panic(err)
				}
				block, err := types.BlockFromProto(pbb)
				if err != nil {
					panic(err)
				}

				if block.Height != height+1 {
					panic(fmt.Sprintf("read bad block from wal. got height %d, expected %d", block.Height, height+1))
				}
				commitHeight := thisBlockCommit.Height
				if commitHeight != height+1 {
					panic(fmt.Sprintf("commit doesnt match. got height %d, expected %d", commitHeight, height+1))
				}
				blocks = append(blocks, block)
				commits = append(commits, thisBlockCommit)
				height++
			}
		case *types.PartSetHeader:
			thisBlockParts = types.NewPartSetFromHeader(*p)
		case *types.Part:
			_, err := thisBlockParts.AddPart(p)
			if err != nil {
				return nil, nil, err
			}
		case *types.Vote:
			if p.Type == tmproto.PrecommitType {
				thisBlockCommit = types.NewCommit(p.Height, p.Round,
					p.BlockID, []types.CommitSig{p.CommitSig()})
			}
		}
	}
	// grab the last block too
	bz, err := io.ReadAll(thisBlockParts.GetReader())
	if err != nil {
		panic(err)
	}
	var pbb = new(tmproto.Block)
	err = proto.Unmarshal(bz, pbb)
	if err != nil {
		panic(err)
	}
	block, err := types.BlockFromProto(pbb)
	if err != nil {
		panic(err)
	}
	if block.Height != height+1 {
		panic(fmt.Sprintf("read bad block from wal. got height %d, expected %d", block.Height, height+1))
	}
	commitHeight := thisBlockCommit.Height
	if commitHeight != height+1 {
		panic(fmt.Sprintf("commit doesnt match. got height %d, expected %d", commitHeight, height+1))
	}
	blocks = append(blocks, block)
	commits = append(commits, thisBlockCommit)
	return blocks, commits, nil
}

func readPieceFromWAL(msg *TimedWALMessage) interface{} {
	// for logging
	switch m := msg.Msg.(type) {
	case msgInfo:
		switch msg := m.Msg.(type) {
		case *ProposalMessage:
			return &msg.Proposal.BlockID.PartSetHeader
		case *BlockPartMessage:
			return msg.Part
		case *VoteMessage:
			return msg.Vote
		}
	case EndHeightMessage:
		return m
	}

	return nil
}

// fresh state and mock store
func stateAndStore(
	t *testing.T,
	config *cfg.Config,
	pubKey crypto.PubKey,
	appVersion uint64,
) (dbm.DB, sm.State, *mockBlockStore) {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardFinalizeBlockResponses: false,
	})
	state, err := sm.MakeGenesisStateFromFile(config.GenesisFile())
	require.NoError(t, err)
	state.Version.Consensus.App = appVersion
	store := newMockBlockStore(t, config, state.ConsensusParams)
	require.NoError(t, stateStore.Save(state))

	return stateDB, state, store
}

//----------------------------------
// mock block store

type mockBlockStore struct {
	config  *cfg.Config
	params  types.ConsensusParams
	chain   []*types.Block
	commits []*types.Commit
	base    int64
	t       *testing.T
}

// TODO: NewBlockStore(db.NewMemDB) ...
func newMockBlockStore(t *testing.T, config *cfg.Config, params types.ConsensusParams) *mockBlockStore {
	return &mockBlockStore{
		config: config,
		params: params,
		t:      t,
	}
}

func (bs *mockBlockStore) Height() int64                       { return int64(len(bs.chain)) }
func (bs *mockBlockStore) Base() int64                         { return bs.base }
func (bs *mockBlockStore) Size() int64                         { return bs.Height() - bs.Base() + 1 }
func (bs *mockBlockStore) LoadBaseMeta() *types.BlockMeta      { return bs.LoadBlockMeta(bs.base) }
func (bs *mockBlockStore) LoadBlock(height int64) *types.Block { return bs.chain[height-1] }
func (bs *mockBlockStore) LoadBlockByHash(hash []byte) *types.Block {
	return bs.chain[int64(len(bs.chain))-1]
}
func (bs *mockBlockStore) LoadBlockMetaByHash(hash []byte) *types.BlockMeta { return nil }
func (bs *mockBlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	block := bs.chain[height-1]
	bps, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(bs.t, err)
	return &types.BlockMeta{
		BlockID: types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()},
		Header:  block.Header,
	}
}
func (bs *mockBlockStore) LoadBlockPart(height int64, index int) *types.Part { return nil }
func (bs *mockBlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
}
func (bs *mockBlockStore) LoadBlockCommit(height int64) *types.Commit {
	return bs.commits[height-1]
}
func (bs *mockBlockStore) LoadSeenCommit(height int64) *types.Commit {
	return bs.commits[height-1]
}

func (bs *mockBlockStore) PruneBlocks(height int64) (uint64, error) {
	pruned := uint64(0)
	for i := int64(0); i < height-1; i++ {
		bs.chain[i] = nil
		bs.commits[i] = nil
		pruned++
	}
	bs.base = height
	return pruned, nil
}

//---------------------------------------
// Test handshake/init chain

func TestHandshakeUpdatesValidators(t *testing.T) {
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})
	app := &mocks.Application{}
	app.On("Info", mock.Anything, mock.Anything).Return(&abci.ResponseInfo{
		LastBlockHeight: 0,
	}, nil)
	app.On("InitChain", mock.Anything, mock.Anything).Return(&abci.ResponseInitChain{
		Validators: types.TM2PB.ValidatorUpdates(vals),
	}, nil)
	clientCreator := proxy.NewLocalClientCreator(app)

	config := ResetConfig("handshake_test_")
	defer os.RemoveAll(config.RootDir)
	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, config, pubKey, 0x0)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardFinalizeBlockResponses: false,
	})

	oldValAddr := state.Validators.Validators[0].Address

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	handshaker := NewHandshaker(stateStore, state, store, genDoc)
	proxyApp := proxy.NewAppConns(clientCreator, proxy.NopMetrics())
	if err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}
	t.Cleanup(func() {
		if err := proxyApp.Stop(); err != nil {
			t.Error(err)
		}
	})
	if err := handshaker.Handshake(proxyApp); err != nil {
		t.Fatalf("Error on abci handshake: %v", err)
	}
	// reload the state, check the validator set was updated
	state, err = stateStore.Load()
	require.NoError(t, err)

	newValAddr := state.Validators.Validators[0].Address
	expectValAddr := val.Address
	assert.NotEqual(t, oldValAddr, newValAddr)
	assert.Equal(t, newValAddr, expectValAddr)
}
