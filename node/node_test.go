package node

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/pubsub"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/libs/service"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

func TestNodeStartStop(t *testing.T) {
	cfg, err := config.ResetTestRoot("node_node_test")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)

	ctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	// create & start node
	ns, err := newDefaultNode(ctx, cfg, log.TestingLogger())
	require.NoError(t, err)
	require.NoError(t, ns.Start(ctx))

	t.Cleanup(func() {
		if ns.IsRunning() {
			bcancel()
			ns.Wait()
		}
	})

	n, ok := ns.(*nodeImpl)
	require.True(t, ok)

	// wait for the node to produce a block
	blocksSub, err := n.EventBus().SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "node_test",
		Query:    types.EventQueryNewBlock,
	})
	require.NoError(t, err)
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := blocksSub.Next(tctx); err != nil {
		t.Fatalf("Waiting for event: %v", err)
	}

	// stop the node
	go func() {
		bcancel()
		n.Wait()
	}()

	select {
	case <-n.Quit():
		return
	case <-time.After(10 * time.Second):
		if n.IsRunning() {
			t.Fatal("timed out waiting for shutdown")
		}

	}
}

func getTestNode(ctx context.Context, t *testing.T, conf *config.Config, logger log.Logger) *nodeImpl {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ns, err := newDefaultNode(ctx, conf, logger)
	require.NoError(t, err)

	n, ok := ns.(*nodeImpl)
	require.True(t, ok)

	t.Cleanup(func() {
		cancel()
		if n.IsRunning() {
			ns.Wait()
		}
	})

	return n
}

func TestNodeDelayedStart(t *testing.T) {
	cfg, err := config.ResetTestRoot("node_delayed_start_test")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)
	now := tmtime.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create & start node
	n := getTestNode(ctx, t, cfg, log.TestingLogger())
	n.GenesisDoc().GenesisTime = now.Add(2 * time.Second)

	require.NoError(t, n.Start(ctx))

	startTime := tmtime.Now()
	assert.Equal(t, true, startTime.After(n.GenesisDoc().GenesisTime))
}

func TestNodeSetAppVersion(t *testing.T) {
	cfg, err := config.ResetTestRoot("node_app_version_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create node
	n := getTestNode(ctx, t, cfg, log.TestingLogger())

	// default config uses the kvstore app
	appVersion := kvstore.ProtocolVersion

	// check version is set in state
	state, err := n.stateStore.Load()
	require.NoError(t, err)
	assert.Equal(t, state.Version.Consensus.App, appVersion)

	// check version is set in node info
	assert.Equal(t, n.nodeInfo.ProtocolVersion.App, appVersion)
}

func TestNodeSetPrivValTCP(t *testing.T) {
	addr := "tcp://" + testFreeAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot("node_priv_val_tcp_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cfg.PrivValidator.ListenAddr = addr

	dialer := privval.DialTCPFn(addr, 100*time.Millisecond, ed25519.GenPrivKey())
	dialerEndpoint := privval.NewSignerDialerEndpoint(
		log.TestingLogger(),
		dialer,
	)
	privval.SignerDialerEndpointTimeoutReadWrite(100 * time.Millisecond)(dialerEndpoint)

	signerServer := privval.NewSignerServer(
		dialerEndpoint,
		cfg.ChainID(),
		types.NewMockPV(),
	)

	go func() {
		err := signerServer.Start(ctx)
		if err != nil {
			panic(err)
		}
	}()
	defer signerServer.Stop() //nolint:errcheck // ignore for tests

	n := getTestNode(ctx, t, cfg, log.TestingLogger())
	assert.IsType(t, &privval.RetrySignerClient{}, n.PrivValidator())
}

// address without a protocol must result in error
func TestPrivValidatorListenAddrNoProtocol(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrNoPrefix := testFreeAddr(t)

	cfg, err := config.ResetTestRoot("node_priv_val_tcp_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cfg.PrivValidator.ListenAddr = addrNoPrefix
	n, err := newDefaultNode(ctx, cfg, log.TestingLogger())

	assert.Error(t, err)

	if n != nil && n.IsRunning() {
		cancel()
		n.Wait()
	}
}

func TestNodeSetPrivValIPC(t *testing.T) {
	tmpfile := "/tmp/kms." + tmrand.Str(6) + ".sock"
	defer os.Remove(tmpfile) // clean up

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot("node_priv_val_tcp_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cfg.PrivValidator.ListenAddr = "unix://" + tmpfile

	dialer := privval.DialUnixFn(tmpfile)
	dialerEndpoint := privval.NewSignerDialerEndpoint(
		log.TestingLogger(),
		dialer,
	)
	privval.SignerDialerEndpointTimeoutReadWrite(100 * time.Millisecond)(dialerEndpoint)

	pvsc := privval.NewSignerServer(
		dialerEndpoint,
		cfg.ChainID(),
		types.NewMockPV(),
	)

	go func() {
		err := pvsc.Start(ctx)
		require.NoError(t, err)
	}()
	defer pvsc.Stop() //nolint:errcheck // ignore for tests
	n := getTestNode(ctx, t, cfg, log.TestingLogger())
	assert.IsType(t, &privval.RetrySignerClient{}, n.PrivValidator())
}

// testFreeAddr claims a free port so we don't block on listener being ready.
func testFreeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}

// create a proposal block using real and full
// mempool and evidence pool and validate it.
func TestCreateProposalBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot("node_create_proposal")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	cc := abciclient.NewLocalCreator(kvstore.NewApplication())
	proxyApp := proxy.NewAppConns(cc, log.TestingLogger(), proxy.NopMetrics())
	err = proxyApp.Start(ctx)
	require.Nil(t, err)

	logger := log.TestingLogger()

	const height int64 = 1
	state, stateDB, privVals := state(t, 1, height)
	stateStore := sm.NewStore(stateDB)
	maxBytes := 16384
	const partSize uint32 = 256
	maxEvidenceBytes := int64(maxBytes / 2)
	state.ConsensusParams.Block.MaxBytes = int64(maxBytes)
	state.ConsensusParams.Evidence.MaxBytes = maxEvidenceBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	mp := mempool.NewTxMempool(
		logger.With("module", "mempool"),
		cfg.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
	)

	// Make EvidencePool
	evidenceDB := dbm.NewMemDB()
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	evidencePool, err := evidence.NewPool(logger, evidenceDB, stateStore, blockStore)
	require.NoError(t, err)

	// fill the evidence pool with more evidence
	// than can fit in a block
	var currentBytes int64
	for currentBytes <= maxEvidenceBytes {
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, time.Now(), privVals[0], "test-chain")
		currentBytes += int64(len(ev.Bytes()))
		evidencePool.ReportConflictingVotes(ev.VoteA, ev.VoteB)
	}

	evList, size := evidencePool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	require.Less(t, size, state.ConsensusParams.Evidence.MaxBytes+1)
	evData := &types.EvidenceData{Evidence: evList}
	require.EqualValues(t, size, evData.ByteSize())

	// fill the mempool with more txs
	// than can fit in a block
	txLength := 100
	for i := 0; i <= maxBytes/txLength; i++ {
		tx := tmrand.Bytes(txLength)
		err := mp.CheckTx(ctx, tx, nil, mempool.TxInfo{})
		assert.NoError(t, err)
	}

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		mp,
		evidencePool,
		blockStore,
	)

	commit := types.NewCommit(height-1, 0, types.BlockID{}, nil)
	block, _ := blockExec.CreateProposalBlock(
		height,
		state, commit,
		proposerAddr,
	)

	// check that the part set does not exceed the maximum block size
	partSet := block.MakePartSet(partSize)
	assert.Less(t, partSet.ByteSize(), int64(maxBytes))

	partSetFromHeader := types.NewPartSetFromHeader(partSet.Header())
	for partSetFromHeader.Count() < partSetFromHeader.Total() {
		added, err := partSetFromHeader.AddPart(partSet.GetPart(int(partSetFromHeader.Count())))
		require.NoError(t, err)
		require.True(t, added)
	}
	assert.EqualValues(t, partSetFromHeader.ByteSize(), partSet.ByteSize())

	err = blockExec.ValidateBlock(state, block)
	assert.NoError(t, err)
}

func TestMaxTxsProposalBlockSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot("node_create_proposal")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)
	cc := abciclient.NewLocalCreator(kvstore.NewApplication())
	proxyApp := proxy.NewAppConns(cc, log.TestingLogger(), proxy.NopMetrics())
	err = proxyApp.Start(ctx)
	require.Nil(t, err)

	logger := log.TestingLogger()

	const height int64 = 1
	state, stateDB, _ := state(t, 1, height)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	const maxBytes int64 = 16384
	const partSize uint32 = 256
	state.ConsensusParams.Block.MaxBytes = maxBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	// Make Mempool

	mp := mempool.NewTxMempool(
		logger.With("module", "mempool"),
		cfg.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
	)

	// fill the mempool with one txs just below the maximum size
	txLength := int(types.MaxDataBytesNoEvidence(maxBytes, 1))
	tx := tmrand.Bytes(txLength - 4) // to account for the varint
	err = mp.CheckTx(ctx, tx, nil, mempool.TxInfo{})
	assert.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
	)

	commit := types.NewCommit(height-1, 0, types.BlockID{}, nil)
	block, _ := blockExec.CreateProposalBlock(
		height,
		state, commit,
		proposerAddr,
	)

	pb, err := block.ToProto()
	require.NoError(t, err)
	assert.Less(t, int64(pb.Size()), maxBytes)

	// check that the part set does not exceed the maximum block size
	partSet := block.MakePartSet(partSize)
	assert.EqualValues(t, partSet.ByteSize(), int64(pb.Size()))
}

func TestMaxProposalBlockSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot("node_create_proposal")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cc := abciclient.NewLocalCreator(kvstore.NewApplication())
	proxyApp := proxy.NewAppConns(cc, log.TestingLogger(), proxy.NopMetrics())
	err = proxyApp.Start(ctx)
	require.Nil(t, err)

	logger := log.TestingLogger()

	state, stateDB, _ := state(t, types.MaxVotesCount, int64(1))
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	const maxBytes int64 = 1024 * 1024 * 2
	state.ConsensusParams.Block.MaxBytes = maxBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	// Make Mempool
	mp := mempool.NewTxMempool(
		logger.With("module", "mempool"),
		cfg.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
	)

	// fill the mempool with one txs just below the maximum size
	txLength := int(types.MaxDataBytesNoEvidence(maxBytes, types.MaxVotesCount))
	tx := tmrand.Bytes(txLength - 6) // to account for the varint
	err = mp.CheckTx(ctx, tx, nil, mempool.TxInfo{})
	assert.NoError(t, err)
	// now produce more txs than what a normal block can hold with 10 smaller txs
	// At the end of the test, only the single big tx should be added
	for i := 0; i < 10; i++ {
		tx := tmrand.Bytes(10)
		err = mp.CheckTx(ctx, tx, nil, mempool.TxInfo{})
		assert.NoError(t, err)
	}

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
	)

	blockID := types.BlockID{
		Hash: tmhash.Sum([]byte("blockID_hash")),
		PartSetHeader: types.PartSetHeader{
			Total: math.MaxInt32,
			Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
		},
	}

	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)
	// change state in order to produce the largest accepted header
	state.LastBlockID = blockID
	state.LastBlockHeight = math.MaxInt64 - 1
	state.LastBlockTime = timestamp
	state.LastResultsHash = tmhash.Sum([]byte("last_results_hash"))
	state.AppHash = tmhash.Sum([]byte("app_hash"))
	state.Version.Consensus.Block = math.MaxInt64
	state.Version.Consensus.App = math.MaxInt64
	maxChainID := ""
	for i := 0; i < types.MaxChainIDLen; i++ {
		maxChainID += "𠜎"
	}
	state.ChainID = maxChainID

	cs := types.CommitSig{
		BlockIDFlag:      types.BlockIDFlagNil,
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		Timestamp:        timestamp,
		Signature:        crypto.CRandBytes(types.MaxSignatureSize),
	}

	commit := &types.Commit{
		Height:  math.MaxInt64,
		Round:   math.MaxInt32,
		BlockID: blockID,
	}

	// add maximum amount of signatures to a single commit
	for i := 0; i < types.MaxVotesCount; i++ {
		commit.Signatures = append(commit.Signatures, cs)
	}

	block, partSet := blockExec.CreateProposalBlock(
		math.MaxInt64,
		state, commit,
		proposerAddr,
	)

	// this ensures that the header is at max size
	block.Header.Time = timestamp

	pb, err := block.ToProto()
	require.NoError(t, err)

	// require that the header and commit be the max possible size
	require.Equal(t, int64(pb.Header.Size()), types.MaxHeaderBytes)
	require.Equal(t, int64(pb.LastCommit.Size()), types.MaxCommitBytes(types.MaxVotesCount))
	// make sure that the block is less than the max possible size
	assert.Equal(t, int64(pb.Size()), maxBytes)
	// because of the proto overhead we expect the part set bytes to be equal or
	// less than the pb block size
	assert.LessOrEqual(t, partSet.ByteSize(), int64(pb.Size()))

}

func TestNodeNewSeedNode(t *testing.T) {
	cfg, err := config.ResetTestRoot("node_new_node_custom_reactors_test")
	require.NoError(t, err)
	cfg.Mode = config.ModeSeed
	defer os.RemoveAll(cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeKey, err := types.LoadOrGenNodeKey(cfg.NodeKeyFile())
	require.NoError(t, err)

	ns, err := makeSeedNode(cfg,
		config.DefaultDBProvider,
		nodeKey,
		defaultGenesisDocProviderFunc(cfg),
		log.TestingLogger(),
	)
	t.Cleanup(ns.Wait)

	require.NoError(t, err)
	n, ok := ns.(*nodeImpl)
	require.True(t, ok)

	err = n.Start(ctx)
	require.NoError(t, err)
	assert.True(t, n.pexReactor.IsRunning())

	cancel()
	n.Wait()

	assert.False(t, n.pexReactor.IsRunning())
}

func TestNodeSetEventSink(t *testing.T) {
	cfg, err := config.ResetTestRoot("node_app_version_test")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.TestingLogger()
	setupTest := func(t *testing.T, conf *config.Config) []indexer.EventSink {
		eventBus, err := createAndStartEventBus(ctx, logger)
		require.NoError(t, err)
		t.Cleanup(eventBus.Wait)
		genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
		require.NoError(t, err)

		indexService, eventSinks, err := createAndStartIndexerService(ctx, cfg,
			config.DefaultDBProvider, eventBus, logger, genDoc.ChainID,
			indexer.NopMetrics())
		require.NoError(t, err)
		t.Cleanup(indexService.Wait)
		return eventSinks
	}
	cleanup := func(ns service.Service) func() {
		return func() {
			n, ok := ns.(*nodeImpl)
			if !ok {
				return
			}
			if n == nil {
				return
			}
			if !n.IsRunning() {
				return
			}
			cancel()
			n.Wait()
		}
	}

	eventSinks := setupTest(t, cfg)
	assert.Equal(t, 1, len(eventSinks))
	assert.Equal(t, indexer.KV, eventSinks[0].Type())

	cfg.TxIndex.Indexer = []string{"null"}
	eventSinks = setupTest(t, cfg)

	assert.Equal(t, 1, len(eventSinks))
	assert.Equal(t, indexer.NULL, eventSinks[0].Type())

	cfg.TxIndex.Indexer = []string{"null", "kv"}
	eventSinks = setupTest(t, cfg)

	assert.Equal(t, 1, len(eventSinks))
	assert.Equal(t, indexer.NULL, eventSinks[0].Type())

	cfg.TxIndex.Indexer = []string{"kvv"}
	ns, err := newDefaultNode(ctx, cfg, logger)
	assert.Nil(t, ns)
	assert.Contains(t, err.Error(), "unsupported event sink type")
	t.Cleanup(cleanup(ns))

	cfg.TxIndex.Indexer = []string{}
	eventSinks = setupTest(t, cfg)

	assert.Equal(t, 1, len(eventSinks))
	assert.Equal(t, indexer.NULL, eventSinks[0].Type())

	cfg.TxIndex.Indexer = []string{"psql"}
	ns, err = newDefaultNode(ctx, cfg, logger)
	assert.Nil(t, ns)
	assert.Contains(t, err.Error(), "the psql connection settings cannot be empty")
	t.Cleanup(cleanup(ns))

	// N.B. We can't create a PSQL event sink without starting a postgres
	// instance for it to talk to. The indexer service tests exercise that case.

	var e = errors.New("found duplicated sinks, please check the tx-index section in the config.toml")
	cfg.TxIndex.Indexer = []string{"null", "kv", "Kv"}
	ns, err = newDefaultNode(ctx, cfg, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), e.Error())
	t.Cleanup(cleanup(ns))

	cfg.TxIndex.Indexer = []string{"Null", "kV", "kv", "nUlL"}
	ns, err = newDefaultNode(ctx, cfg, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), e.Error())
	t.Cleanup(cleanup(ns))
}

func state(t *testing.T, nVals int, height int64) (sm.State, dbm.DB, []types.PrivValidator) {
	t.Helper()
	privVals := make([]types.PrivValidator, nVals)
	vals := make([]types.GenesisValidator, nVals)
	for i := 0; i < nVals; i++ {
		privVal := types.NewMockPV()
		privVals[i] = privVal
		vals[i] = types.GenesisValidator{
			Address: privVal.PrivKey.PubKey().Address(),
			PubKey:  privVal.PrivKey.PubKey(),
			Power:   1000,
			Name:    fmt.Sprintf("test%d", i),
		}
	}
	s, _ := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID:    "test-chain",
		Validators: vals,
		AppHash:    nil,
	})

	// save validators to db for 2 heights
	stateDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, stateDB.Close()) })

	stateStore := sm.NewStore(stateDB)
	require.NoError(t, stateStore.Save(s))

	for i := 1; i < int(height); i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()
		require.NoError(t, stateStore.Save(s))
	}
	return s, stateDB, privVals
}

func TestLoadStateFromGenesis(t *testing.T) {
	_ = loadStatefromGenesis(t)
}

func loadStatefromGenesis(t *testing.T) sm.State {
	t.Helper()

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	cfg, err := config.ResetTestRoot("load_state_from_genesis")
	require.NoError(t, err)

	loadedState, err := stateStore.Load()
	require.NoError(t, err)
	require.True(t, loadedState.IsEmpty())

	genDoc, _ := factory.RandGenesisDoc(cfg, 0, false, 10)

	state, err := loadStateFromDBOrGenesisDocProvider(
		stateStore,
		genDoc,
	)
	require.NoError(t, err)
	require.NotNil(t, state)

	return state
}
