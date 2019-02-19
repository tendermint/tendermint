package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	auto "github.com/tendermint/tendermint/libs/autofile"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
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

func startNewConsensusStateAndWaitForBlock(t *testing.T, consensusReplayConfig *cfg.Config,
	lastBlockHeight int64, blockDB dbm.DB, stateDB dbm.DB) {
	logger := log.TestingLogger()
	state, _ := sm.LoadStateFromDBOrGenesisFile(stateDB, consensusReplayConfig.GenesisFile())
	privValidator := loadPrivValidator(consensusReplayConfig)
	cs := newConsensusStateWithConfigAndBlockStore(consensusReplayConfig, state, privValidator, kvstore.NewKVStoreApplication(), blockDB)
	cs.SetLogger(logger)

	bytes, _ := ioutil.ReadFile(cs.config.WalFile())
	t.Logf("====== WAL: \n\r%X\n", bytes)

	err := cs.Start()
	require.NoError(t, err)
	defer cs.Stop()

	// This is just a signal that we haven't halted; its not something contained
	// in the WAL itself. Assuming the consensus state is running, replay of any
	// WAL, including the empty one, should eventually be followed by a new
	// block, or else something is wrong.
	newBlockCh := make(chan interface{}, 1)
	err = cs.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock, newBlockCh)
	require.NoError(t, err)
	select {
	case <-newBlockCh:
	case <-time.After(60 * time.Second):
		t.Fatalf("Timed out waiting for new block (see trace above)")
	}
}

func sendTxs(cs *ConsensusState, ctx context.Context) {
	for i := 0; i < 256; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			tx := []byte{byte(i)}
			assertMempool(cs.txNotifier).CheckTx(tx, nil)
			i++
		}
	}
}

// TestWALCrash uses crashing WAL to test we can recover from any WAL failure.
func TestWALCrash(t *testing.T) {
	testCases := []struct {
		name         string
		initFn       func(dbm.DB, *ConsensusState, context.Context)
		heightToStop int64
	}{
		{"empty block",
			func(stateDB dbm.DB, cs *ConsensusState, ctx context.Context) {},
			1},
		{"many non-empty blocks",
			func(stateDB dbm.DB, cs *ConsensusState, ctx context.Context) {
				go sendTxs(cs, ctx)
			},
			3},
	}

	for i, tc := range testCases {
		consensusReplayConfig := ResetConfig(fmt.Sprintf("%s_%d", t.Name(), i))
		t.Run(tc.name, func(t *testing.T) {
			crashWALandCheckLiveness(t, consensusReplayConfig, tc.initFn, tc.heightToStop)
		})
	}
}

func crashWALandCheckLiveness(t *testing.T, consensusReplayConfig *cfg.Config,
	initFn func(dbm.DB, *ConsensusState, context.Context), heightToStop int64) {
	walPaniced := make(chan error)
	crashingWal := &crashingWAL{panicCh: walPaniced, heightToStop: heightToStop}

	i := 1
LOOP:
	for {
		t.Logf("====== LOOP %d\n", i)

		// create consensus state from a clean slate
		logger := log.NewNopLogger()
		stateDB := dbm.NewMemDB()
		state, _ := sm.MakeGenesisStateFromFile(consensusReplayConfig.GenesisFile())
		privValidator := loadPrivValidator(consensusReplayConfig)
		blockDB := dbm.NewMemDB()
		cs := newConsensusStateWithConfigAndBlockStore(consensusReplayConfig, state, privValidator, kvstore.NewKVStoreApplication(), blockDB)
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
		case err := <-walPaniced:
			t.Logf("WAL paniced: %v", err)

			// make sure we can make blocks after a crash
			startNewConsensusStateAndWaitForBlock(t, consensusReplayConfig, cs.Height, blockDB, stateDB)

			// stop consensus state and transactions sender (initFn)
			cs.Stop()
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
// (lastPanicedForMsgIndex), so we don't panic for it in subsequent iterations.
type crashingWAL struct {
	next         WAL
	panicCh      chan error
	heightToStop int64

	msgIndex               int // current message index
	lastPanicedForMsgIndex int // last message for which we panicked
}

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
func (w *crashingWAL) Write(m WALMessage) {
	if endMsg, ok := m.(EndHeightMessage); ok {
		if endMsg.Height == w.heightToStop {
			w.panicCh <- ReachedHeightToStopError{endMsg.Height}
			runtime.Goexit()
		} else {
			w.next.Write(m)
		}
		return
	}

	if w.msgIndex > w.lastPanicedForMsgIndex {
		w.lastPanicedForMsgIndex = w.msgIndex
		_, file, line, _ := runtime.Caller(1)
		w.panicCh <- WALWriteError{fmt.Sprintf("failed to write %T to WAL (fileline: %s:%d)", m, file, line)}
		runtime.Goexit()
	} else {
		w.msgIndex++
		w.next.Write(m)
	}
}

func (w *crashingWAL) WriteSync(m WALMessage) {
	w.Write(m)
}

func (w *crashingWAL) Group() *auto.Group { return w.next.Group() }
func (w *crashingWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (gr *auto.GroupReader, found bool, err error) {
	return w.next.SearchForEndHeight(height, options)
}

func (w *crashingWAL) Start() error { return w.next.Start() }
func (w *crashingWAL) Stop() error  { return w.next.Stop() }
func (w *crashingWAL) Wait()        { w.next.Wait() }

//------------------------------------------------------------------------------------------
// Handshake Tests

const (
	NUM_BLOCKS = 6
)

var (
	mempool = sm.MockMempool{}
	evpool  = sm.MockEvidencePool{}
)

//---------------------------------------
// Test handshake/replay

// 0 - all synced up
// 1 - saved block but app and state are behind
// 2 - save block and committed but state is behind
var modes = []uint{0, 1, 2}

// Sync from scratch
func TestHandshakeReplayAll(t *testing.T) {
	for i, m := range modes {
		config := ResetConfig(fmt.Sprintf("%s_%v", t.Name(), i))
		defer os.RemoveAll(config.RootDir)
		testHandshakeReplay(t, config, 0, m)
	}
}

// Sync many, not from scratch
func TestHandshakeReplaySome(t *testing.T) {
	for i, m := range modes {
		config := ResetConfig(fmt.Sprintf("%s_%v", t.Name(), i))
		defer os.RemoveAll(config.RootDir)
		testHandshakeReplay(t, config, 1, m)
	}
}

// Sync from lagging by one
func TestHandshakeReplayOne(t *testing.T) {
	for i, m := range modes {
		config := ResetConfig(fmt.Sprintf("%s_%v", t.Name(), i))
		defer os.RemoveAll(config.RootDir)
		testHandshakeReplay(t, config, NUM_BLOCKS-1, m)
	}
}

// Sync from caught up
func TestHandshakeReplayNone(t *testing.T) {
	for i, m := range modes {
		config := ResetConfig(fmt.Sprintf("%s_%v", t.Name(), i))
		defer os.RemoveAll(config.RootDir)
		testHandshakeReplay(t, config, NUM_BLOCKS, m)
	}
}

func tempWALWithData(data []byte) string {
	walFile, err := ioutil.TempFile("", "wal")
	if err != nil {
		panic(fmt.Errorf("failed to create temp WAL file: %v", err))
	}
	_, err = walFile.Write(data)
	if err != nil {
		panic(fmt.Errorf("failed to write to temp WAL file: %v", err))
	}
	if err := walFile.Close(); err != nil {
		panic(fmt.Errorf("failed to close temp WAL file: %v", err))
	}
	return walFile.Name()
}

// Make some blocks. Start a fresh app and apply nBlocks blocks. Then restart the app and sync it up with the remaining blocks
func testHandshakeReplay(t *testing.T, config *cfg.Config, nBlocks int, mode uint) {
	walBody, err := WALWithNBlocks(t, NUM_BLOCKS)
	require.NoError(t, err)
	walFile := tempWALWithData(walBody)
	config.Consensus.SetWalFile(walFile)

	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())

	wal, err := NewWAL(walFile)
	require.NoError(t, err)
	wal.SetLogger(log.TestingLogger())
	err = wal.Start()
	require.NoError(t, err)
	defer wal.Stop()

	chain, commits, err := makeBlockchainFromWAL(wal)
	require.NoError(t, err)

	stateDB, state, store := stateAndStore(config, privVal.GetPubKey(), kvstore.ProtocolVersion)
	store.chain = chain
	store.commits = commits

	// run the chain through state.ApplyBlock to build up the tendermint state
	state = buildTMStateFromChain(config, stateDB, state, chain, mode)
	latestAppHash := state.AppHash

	// make a new client creator
	kvstoreApp := kvstore.NewPersistentKVStoreApplication(path.Join(config.DBDir(), "2"))
	clientCreator2 := proxy.NewLocalClientCreator(kvstoreApp)
	if nBlocks > 0 {
		// run nBlocks against a new client to build up the app state.
		// use a throwaway tendermint state
		proxyApp := proxy.NewAppConns(clientCreator2)
		stateDB, state, _ := stateAndStore(config, privVal.GetPubKey(), kvstore.ProtocolVersion)
		buildAppStateFromChain(proxyApp, stateDB, state, chain, nBlocks, mode)
	}

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	handshaker := NewHandshaker(stateDB, state, store, genDoc)
	proxyApp := proxy.NewAppConns(clientCreator2)
	if err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}
	defer proxyApp.Stop()
	if err := handshaker.Handshake(proxyApp); err != nil {
		t.Fatalf("Error on abci handshake: %v", err)
	}

	// get the latest app hash from the app
	res, err := proxyApp.Query().InfoSync(abci.RequestInfo{Version: ""})
	if err != nil {
		t.Fatal(err)
	}

	// the app hash should be synced up
	if !bytes.Equal(latestAppHash, res.LastBlockAppHash) {
		t.Fatalf("Expected app hashes to match after handshake/replay. got %X, expected %X", res.LastBlockAppHash, latestAppHash)
	}

	expectedBlocksToSync := NUM_BLOCKS - nBlocks
	if nBlocks == NUM_BLOCKS && mode > 0 {
		expectedBlocksToSync++
	} else if nBlocks > 0 && mode == 1 {
		expectedBlocksToSync++
	}

	if handshaker.NBlocks() != expectedBlocksToSync {
		t.Fatalf("Expected handshake to sync %d blocks, got %d", expectedBlocksToSync, handshaker.NBlocks())
	}
}

func applyBlock(stateDB dbm.DB, st sm.State, blk *types.Block, proxyApp proxy.AppConns) sm.State {
	testPartSize := types.BlockPartSizeBytes
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mempool, evpool)

	blkID := types.BlockID{blk.Hash(), blk.MakePartSet(testPartSize).Header()}
	newState, err := blockExec.ApplyBlock(st, blkID, blk)
	if err != nil {
		panic(err)
	}
	return newState
}

func buildAppStateFromChain(proxyApp proxy.AppConns, stateDB dbm.DB,
	state sm.State, chain []*types.Block, nBlocks int, mode uint) {
	// start a new app without handshake, play nBlocks blocks
	if err := proxyApp.Start(); err != nil {
		panic(err)
	}
	defer proxyApp.Stop()

	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	if _, err := proxyApp.Consensus().InitChainSync(abci.RequestInitChain{
		Validators: validators,
	}); err != nil {
		panic(err)
	}

	switch mode {
	case 0:
		for i := 0; i < nBlocks; i++ {
			block := chain[i]
			state = applyBlock(stateDB, state, block, proxyApp)
		}
	case 1, 2:
		for i := 0; i < nBlocks-1; i++ {
			block := chain[i]
			state = applyBlock(stateDB, state, block, proxyApp)
		}

		if mode == 2 {
			// update the kvstore height and apphash
			// as if we ran commit but not
			state = applyBlock(stateDB, state, chain[nBlocks-1], proxyApp)
		}
	}

}

func buildTMStateFromChain(config *cfg.Config, stateDB dbm.DB, state sm.State, chain []*types.Block, mode uint) sm.State {
	// run the whole chain against this client to build up the tendermint state
	clientCreator := proxy.NewLocalClientCreator(kvstore.NewPersistentKVStoreApplication(path.Join(config.DBDir(), "1")))
	proxyApp := proxy.NewAppConns(clientCreator)
	if err := proxyApp.Start(); err != nil {
		panic(err)
	}
	defer proxyApp.Stop()

	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	if _, err := proxyApp.Consensus().InitChainSync(abci.RequestInitChain{
		Validators: validators,
	}); err != nil {
		panic(err)
	}

	switch mode {
	case 0:
		// sync right up
		for _, block := range chain {
			state = applyBlock(stateDB, state, block, proxyApp)
		}

	case 1, 2:
		// sync up to the penultimate as if we stored the block.
		// whether we commit or not depends on the appHash
		for _, block := range chain[:len(chain)-1] {
			state = applyBlock(stateDB, state, block, proxyApp)
		}

		// apply the final block to a state copy so we can
		// get the right next appHash but keep the state back
		applyBlock(stateDB, state, chain[len(chain)-1], proxyApp)
	}

	return state
}

//--------------------------
// utils for making blocks

func makeBlockchainFromWAL(wal WAL) ([]*types.Block, []*types.Commit, error) {
	// Search for height marker
	gr, found, err := wal.SearchForEndHeight(0, &WALSearchOptions{})
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("WAL does not contain height %d.", 1)
	}
	defer gr.Close() // nolint: errcheck

	// log.Notice("Build a blockchain by reading from the WAL")

	var blocks []*types.Block
	var commits []*types.Commit

	var thisBlockParts *types.PartSet
	var thisBlockCommit *types.Commit
	var height int64

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
				var block = new(types.Block)
				_, err = cdc.UnmarshalBinaryLengthPrefixedReader(thisBlockParts.GetReader(), block, 0)
				if err != nil {
					panic(err)
				}
				if block.Height != height+1 {
					panic(fmt.Sprintf("read bad block from wal. got height %d, expected %d", block.Height, height+1))
				}
				commitHeight := thisBlockCommit.Precommits[0].Height
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
			if p.Type == types.PrecommitType {
				commitSigs := []*types.CommitSig{p.CommitSig()}
				thisBlockCommit = types.NewCommit(p.BlockID, commitSigs)
			}
		}
	}
	// grab the last block too
	var block = new(types.Block)
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(thisBlockParts.GetReader(), block, 0)
	if err != nil {
		panic(err)
	}
	if block.Height != height+1 {
		panic(fmt.Sprintf("read bad block from wal. got height %d, expected %d", block.Height, height+1))
	}
	commitHeight := thisBlockCommit.Precommits[0].Height
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
			return &msg.Proposal.BlockID.PartsHeader
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
func stateAndStore(config *cfg.Config, pubKey crypto.PubKey, appVersion version.Protocol) (dbm.DB, sm.State, *mockBlockStore) {
	stateDB := dbm.NewMemDB()
	state, _ := sm.MakeGenesisStateFromFile(config.GenesisFile())
	state.Version.Consensus.App = appVersion
	store := NewMockBlockStore(config, state.ConsensusParams)
	return stateDB, state, store
}

//----------------------------------
// mock block store

type mockBlockStore struct {
	config  *cfg.Config
	params  types.ConsensusParams
	chain   []*types.Block
	commits []*types.Commit
}

// TODO: NewBlockStore(db.NewMemDB) ...
func NewMockBlockStore(config *cfg.Config, params types.ConsensusParams) *mockBlockStore {
	return &mockBlockStore{config, params, nil, nil}
}

func (bs *mockBlockStore) Height() int64                       { return int64(len(bs.chain)) }
func (bs *mockBlockStore) LoadBlock(height int64) *types.Block { return bs.chain[height-1] }
func (bs *mockBlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	block := bs.chain[height-1]
	return &types.BlockMeta{
		BlockID: types.BlockID{block.Hash(), block.MakePartSet(types.BlockPartSizeBytes).Header()},
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

//----------------------------------------

func TestInitChainUpdateValidators(t *testing.T) {
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})
	app := &initChainApp{vals: types.TM2PB.ValidatorUpdates(vals)}
	clientCreator := proxy.NewLocalClientCreator(app)

	config := ResetConfig("proxy_test_")
	defer os.RemoveAll(config.RootDir)
	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	stateDB, state, store := stateAndStore(config, privVal.GetPubKey(), 0x0)

	oldValAddr := state.Validators.Validators[0].Address

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	handshaker := NewHandshaker(stateDB, state, store, genDoc)
	proxyApp := proxy.NewAppConns(clientCreator)
	if err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}
	defer proxyApp.Stop()
	if err := handshaker.Handshake(proxyApp); err != nil {
		t.Fatalf("Error on abci handshake: %v", err)
	}

	// reload the state, check the validator set was updated
	state = sm.LoadState(stateDB)

	newValAddr := state.Validators.Validators[0].Address
	expectValAddr := val.Address
	assert.NotEqual(t, oldValAddr, newValAddr)
	assert.Equal(t, newValAddr, expectValAddr)
}

// returns the vals on InitChain
type initChainApp struct {
	abci.BaseApplication
	vals []abci.ValidatorUpdate
}

func (ica *initChainApp) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	return abci.ResponseInitChain{
		Validators: ica.vals,
	}
}
