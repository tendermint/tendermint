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

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/libs/service"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestNodeStartStop(t *testing.T) {
	cfg, err := config.ResetTestRoot(t.TempDir(), "node_node_test")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)

	ctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	logger := log.NewNopLogger()
	// create & start node
	ns, err := newDefaultNode(ctx, cfg, logger)
	require.NoError(t, err)

	n, ok := ns.(*nodeImpl)
	require.True(t, ok)
	t.Cleanup(func() {
		bcancel()
		n.Wait()
	})
	t.Cleanup(leaktest.CheckTimeout(t, time.Second))

	require.NoError(t, n.Start(ctx))
	// wait for the node to produce a block
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	blocksSub, err := n.EventBus().SubscribeWithArgs(tctx, pubsub.SubscribeArgs{
		ClientID: "node_test",
		Query:    types.EventQueryNewBlock,
		Limit:    1000,
	})
	require.NoError(t, err)
	_, err = blocksSub.Next(tctx)
	require.NoError(t, err, "waiting for event")

	cancel()  // stop the subscription context
	bcancel() // stop the base context
	n.Wait()

	require.False(t, n.IsRunning(), "node must shut down")
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

	t.Cleanup(leaktest.CheckTimeout(t, time.Second))

	return n
}

func TestNodeDelayedStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_delayed_start_test")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)
	now := tmtime.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	// create & start node
	n := getTestNode(ctx, t, cfg, logger)
	n.GenesisDoc().GenesisTime = now.Add(2 * time.Second)

	require.NoError(t, n.Start(ctx))

	startTime := tmtime.Now()
	assert.Equal(t, true, startTime.After(n.GenesisDoc().GenesisTime))
}

func TestNodeSetAppVersion(t *testing.T) {
	cfg, err := config.ResetTestRoot(t.TempDir(), "node_app_version_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	// create node
	n := getTestNode(ctx, t, cfg, logger)

	require.NoError(t, n.Start(ctx))

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

	t.Cleanup(leaktest.Check(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_priv_val_tcp_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cfg.PrivValidator.ListenAddr = addr

	dialer := privval.DialTCPFn(addr, 100*time.Millisecond, ed25519.GenPrivKey())
	dialerEndpoint := privval.NewSignerDialerEndpoint(logger, dialer)
	privval.SignerDialerEndpointTimeoutReadWrite(100 * time.Millisecond)(dialerEndpoint)

	signerServer := privval.NewSignerServer(
		dialerEndpoint,
		cfg.ChainID(),
		types.NewMockPV(),
	)

	go func() {
		err := signerServer.Start(ctx)
		require.NoError(t, err)
	}()
	defer signerServer.Stop()

	genDoc, err := defaultGenesisDocProviderFunc(cfg)()
	require.NoError(t, err)

	pval, err := createPrivval(ctx, logger, cfg, genDoc, nil)
	require.NoError(t, err)

	assert.IsType(t, &privval.RetrySignerClient{}, pval)
}

// address without a protocol must result in error
func TestPrivValidatorListenAddrNoProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrNoPrefix := testFreeAddr(t)

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_priv_val_tcp_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cfg.PrivValidator.ListenAddr = addrNoPrefix

	logger := log.NewNopLogger()

	n, err := newDefaultNode(ctx, cfg, logger)

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

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_priv_val_tcp_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	cfg.PrivValidator.ListenAddr = "unix://" + tmpfile

	logger := log.NewNopLogger()

	dialer := privval.DialUnixFn(tmpfile)
	dialerEndpoint := privval.NewSignerDialerEndpoint(logger, dialer)

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
	defer pvsc.Stop()
	genDoc, err := defaultGenesisDocProviderFunc(cfg)()
	require.NoError(t, err)

	pval, err := createPrivval(ctx, logger, cfg, genDoc, nil)
	require.NoError(t, err)

	assert.IsType(t, &privval.RetrySignerClient{}, pval)
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

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_create_proposal")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	logger := log.NewNopLogger()

	cc := abciclient.NewLocalClient(logger, kvstore.NewApplication())
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err = proxyApp.Start(ctx)
	require.NoError(t, err)

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
		proxyApp,
	)

	// Make EvidencePool
	evidenceDB := dbm.NewMemDB()
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	evidencePool := evidence.NewPool(logger, evidenceDB, stateStore, blockStore, evidence.NopMetrics(), nil)

	// fill the evidence pool with more evidence
	// than can fit in a block
	var currentBytes int64
	for currentBytes <= maxEvidenceBytes {
		ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, height, time.Now(), privVals[0], "test-chain")
		require.NoError(t, err)
		currentBytes += int64(len(ev.Bytes()))
		evidencePool.ReportConflictingVotes(ev.VoteA, ev.VoteB)
	}

	evList, size := evidencePool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	require.Less(t, size, state.ConsensusParams.Evidence.MaxBytes+1)
	evData := types.EvidenceList(evList)
	require.EqualValues(t, size, evData.ByteSize())

	// fill the mempool with more txs
	// than can fit in a block
	txLength := 100
	for i := 0; i <= maxBytes/txLength; i++ {
		tx := tmrand.Bytes(txLength)
		err := mp.CheckTx(ctx, tx, nil, mempool.TxInfo{})
		assert.NoError(t, err)
	}

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evidencePool,
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	extCommit := &types.ExtendedCommit{Height: height - 1}
	block, err := blockExec.CreateProposalBlock(
		ctx,
		height,
		state,
		extCommit,
		proposerAddr,
	)
	require.NoError(t, err)

	// check that the part set does not exceed the maximum block size
	partSet, err := block.MakePartSet(partSize)
	require.NoError(t, err)
	assert.Less(t, partSet.ByteSize(), int64(maxBytes))

	partSetFromHeader := types.NewPartSetFromHeader(partSet.Header())
	for partSetFromHeader.Count() < partSetFromHeader.Total() {
		added, err := partSetFromHeader.AddPart(partSet.GetPart(int(partSetFromHeader.Count())))
		require.NoError(t, err)
		require.True(t, added)
	}
	assert.EqualValues(t, partSetFromHeader.ByteSize(), partSet.ByteSize())

	err = blockExec.ValidateBlock(ctx, state, block)
	assert.NoError(t, err)
}

func TestMaxTxsProposalBlockSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_create_proposal")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)

	logger := log.NewNopLogger()

	cc := abciclient.NewLocalClient(logger, kvstore.NewApplication())
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err = proxyApp.Start(ctx)
	require.NoError(t, err)

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
		proxyApp,
	)

	// fill the mempool with one txs just below the maximum size
	txLength := int(types.MaxDataBytesNoEvidence(maxBytes, 1))
	tx := tmrand.Bytes(txLength - 4) // to account for the varint
	err = mp.CheckTx(ctx, tx, nil, mempool.TxInfo{})
	assert.NoError(t, err)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	extCommit := &types.ExtendedCommit{Height: height - 1}
	block, err := blockExec.CreateProposalBlock(
		ctx,
		height,
		state,
		extCommit,
		proposerAddr,
	)
	require.NoError(t, err)

	pb, err := block.ToProto()
	require.NoError(t, err)
	assert.Less(t, int64(pb.Size()), maxBytes)

	// check that the part set does not exceed the maximum block size
	partSet, err := block.MakePartSet(partSize)
	require.NoError(t, err)
	assert.EqualValues(t, partSet.ByteSize(), int64(pb.Size()))
}

func TestMaxProposalBlockSize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "node_create_proposal")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	logger := log.NewNopLogger()

	cc := abciclient.NewLocalClient(logger, kvstore.NewApplication())
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err = proxyApp.Start(ctx)
	require.NoError(t, err)

	state, stateDB, privVals := state(t, types.MaxVotesCount, int64(1))

	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	const maxBytes int64 = 1024 * 1024 * 2
	state.ConsensusParams.Block.MaxBytes = maxBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	// Make Mempool
	mp := mempool.NewTxMempool(
		logger.With("module", "mempool"),
		cfg.Mempool,
		proxyApp,
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

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	blockID := types.BlockID{
		Hash: crypto.Checksum([]byte("blockID_hash")),
		PartSetHeader: types.PartSetHeader{
			Total: math.MaxInt32,
			Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
		},
	}

	// save the updated validator set for use by the block executor.
	state.LastBlockHeight = math.MaxInt64 - 3
	state.LastHeightValidatorsChanged = math.MaxInt64 - 1
	state.NextValidators = state.Validators.Copy()
	require.NoError(t, stateStore.Save(state))

	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)
	// change state in order to produce the largest accepted header
	state.LastBlockID = blockID
	state.LastBlockHeight = math.MaxInt64 - 2
	state.LastBlockTime = timestamp
	state.LastResultsHash = crypto.Checksum([]byte("last_results_hash"))
	state.AppHash = crypto.Checksum([]byte("app_hash"))
	state.Version.Consensus.Block = math.MaxInt64
	state.Version.Consensus.App = math.MaxInt64
	maxChainID := ""
	for i := 0; i < types.MaxChainIDLen; i++ {
		maxChainID += "𠜎"
	}
	state.ChainID = maxChainID

	voteSet := types.NewExtendedVoteSet(state.ChainID, math.MaxInt64-1, math.MaxInt32, tmproto.PrecommitType, state.Validators)

	// add maximum amount of signatures to a single commit
	for i := 0; i < types.MaxVotesCount; i++ {
		pubKey, err := privVals[i].GetPubKey(ctx)
		require.NoError(t, err)
		valIdx, val := state.Validators.GetByAddress(pubKey.Address())
		require.NotNil(t, val)

		vote := &types.Vote{
			Type:             tmproto.PrecommitType,
			Height:           math.MaxInt64 - 1,
			Round:            math.MaxInt32,
			BlockID:          blockID,
			Timestamp:        timestamp,
			ValidatorAddress: val.Address,
			ValidatorIndex:   valIdx,
			Extension:        []byte("extension"),
		}
		vpb := vote.ToProto()
		require.NoError(t, privVals[i].SignVote(ctx, state.ChainID, vpb))
		vote.Signature = vpb.Signature
		vote.ExtensionSignature = vpb.ExtensionSignature

		added, err := voteSet.AddVote(vote)
		require.NoError(t, err)
		require.True(t, added)
	}

	block, err := blockExec.CreateProposalBlock(
		ctx,
		math.MaxInt64,
		state,
		voteSet.MakeExtendedCommit(),
		proposerAddr,
	)
	require.NoError(t, err)
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)

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
	cfg, err := config.ResetTestRoot(t.TempDir(), "node_new_node_custom_reactors_test")
	require.NoError(t, err)
	cfg.Mode = config.ModeSeed
	defer os.RemoveAll(cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeKey, err := types.LoadOrGenNodeKey(cfg.NodeKeyFile())
	require.NoError(t, err)

	logger := log.NewNopLogger()

	ns, err := makeSeedNode(
		logger,
		cfg,
		config.DefaultDBProvider,
		nodeKey,
		defaultGenesisDocProviderFunc(cfg),
	)
	t.Cleanup(ns.Wait)
	t.Cleanup(leaktest.CheckTimeout(t, time.Second))

	require.NoError(t, err)
	n, ok := ns.(*seedNodeImpl)
	require.True(t, ok)

	err = n.Start(ctx)
	require.NoError(t, err)
	assert.True(t, n.pexReactor.IsRunning())

	cancel()
	n.Wait()

	assert.False(t, n.pexReactor.IsRunning())
}

func TestNodeSetEventSink(t *testing.T) {
	cfg, err := config.ResetTestRoot(t.TempDir(), "node_app_version_test")
	require.NoError(t, err)

	defer os.RemoveAll(cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	setupTest := func(t *testing.T, conf *config.Config) []indexer.EventSink {
		eventBus := eventbus.NewDefault(logger.With("module", "events"))
		require.NoError(t, eventBus.Start(ctx))

		t.Cleanup(eventBus.Wait)
		genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
		require.NoError(t, err)

		eventSinks, err := sink.EventSinksFromConfig(cfg, config.DefaultDBProvider, genDoc.ChainID)
		require.NoError(t, err)

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = loadStatefromGenesis(ctx, t)
}

func loadStatefromGenesis(ctx context.Context, t *testing.T) sm.State {
	t.Helper()

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	cfg, err := config.ResetTestRoot(t.TempDir(), "load_state_from_genesis")
	require.NoError(t, err)

	loadedState, err := stateStore.Load()
	require.NoError(t, err)
	require.True(t, loadedState.IsEmpty())

	valSet, _ := factory.ValidatorSet(ctx, t, 0, 10)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())

	state, err := loadStateFromDBOrGenesisDocProvider(
		stateStore,
		genDoc,
	)
	require.NoError(t, err)
	require.NotNil(t, state)

	return state
}
