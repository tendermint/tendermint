package node

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	p2pmock "github.com/tendermint/tendermint/p2p/mock"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestNodeStartStop(t *testing.T) {
	config := cfg.ResetTestRoot("node_node_test")
	defer os.RemoveAll(config.RootDir)

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)
	err = n.Start()
	require.NoError(t, err)

	t.Logf("Started node %v", n.sw.NodeInfo())

	// wait for the node to produce a block
	blocksSub, err := n.EventBus().Subscribe(context.Background(), "node_test", types.EventQueryNewBlock)
	require.NoError(t, err)
	select {
	case <-blocksSub.Out():
	case <-blocksSub.Canceled():
		t.Fatal("blocksSub was canceled")
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the node to produce a block")
	}

	// stop the node
	go func() {
		err = n.Stop()
		require.NoError(t, err)
	}()

	select {
	case <-n.Quit():
	case <-time.After(5 * time.Second):
		pid := os.Getpid()
		p, err := os.FindProcess(pid)
		if err != nil {
			panic(err)
		}
		err = p.Signal(syscall.SIGABRT)
		fmt.Println(err)
		t.Fatal("timed out waiting for shutdown")
	}
}

func TestSplitAndTrimEmpty(t *testing.T) {
	testCases := []struct {
		s        string
		sep      string
		cutset   string
		expected []string
	}{
		{"a,b,c", ",", " ", []string{"a", "b", "c"}},
		{" a , b , c ", ",", " ", []string{"a", "b", "c"}},
		{" a, b, c ", ",", " ", []string{"a", "b", "c"}},
		{" a, ", ",", " ", []string{"a"}},
		{"   ", ",", " ", []string{}},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, splitAndTrimEmpty(tc.s, tc.sep, tc.cutset), "%s", tc.s)
	}
}

func TestNodeDelayedStart(t *testing.T) {
	config := cfg.ResetTestRoot("node_delayed_start_test")
	defer os.RemoveAll(config.RootDir)
	now := tmtime.Now()

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	n.GenesisDoc().GenesisTime = now.Add(2 * time.Second)
	require.NoError(t, err)

	err = n.Start()
	require.NoError(t, err)
	defer n.Stop() //nolint:errcheck // ignore for tests

	startTime := tmtime.Now()
	assert.Equal(t, true, startTime.After(n.GenesisDoc().GenesisTime))
}

func TestNodeSetAppVersion(t *testing.T) {
	config := cfg.ResetTestRoot("node_app_version_test")
	defer os.RemoveAll(config.RootDir)

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)

	// default config uses the kvstore app
	var appVersion uint64 = kvstore.ProtocolVersion

	// check version is set in state
	state, err := n.stateStore.Load()
	require.NoError(t, err)
	assert.Equal(t, state.Version.Consensus.App, appVersion)

	// check version is set in node info
	assert.Equal(t, n.nodeInfo.ProtocolVersion.App, appVersion)
}

func TestNodeSetPrivValTCP(t *testing.T) {
	addr := "tcp://" + testFreeAddr(t)

	config := cfg.ResetTestRoot("node_priv_val_tcp_test")
	defer os.RemoveAll(config.RootDir)
	config.BaseConfig.PrivValidatorListenAddr = addr

	dialer := privval.DialTCPFn(addr, 100*time.Millisecond, ed25519.GenPrivKey())
	dialerEndpoint := privval.NewSignerDialerEndpoint(
		log.TestingLogger(),
		dialer,
	)
	privval.SignerDialerEndpointTimeoutReadWrite(100 * time.Millisecond)(dialerEndpoint)

	signerServer := privval.NewSignerServer(
		dialerEndpoint,
		config.ChainID(),
		types.NewMockPV(),
	)

	go func() {
		err := signerServer.Start()
		if err != nil {
			panic(err)
		}
	}()
	defer signerServer.Stop() //nolint:errcheck // ignore for tests

	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)
	assert.IsType(t, &privval.RetrySignerClient{}, n.PrivValidator())
}

// address without a protocol must result in error
func TestPrivValidatorListenAddrNoProtocol(t *testing.T) {
	addrNoPrefix := testFreeAddr(t)

	config := cfg.ResetTestRoot("node_priv_val_tcp_test")
	defer os.RemoveAll(config.RootDir)
	config.BaseConfig.PrivValidatorListenAddr = addrNoPrefix

	_, err := DefaultNewNode(config, log.TestingLogger())
	assert.Error(t, err)
}

func TestNodeSetPrivValIPC(t *testing.T) {
	tmpfile := "/tmp/kms." + tmrand.Str(6) + ".sock"
	defer os.Remove(tmpfile) // clean up

	config := cfg.ResetTestRoot("node_priv_val_tcp_test")
	defer os.RemoveAll(config.RootDir)
	config.BaseConfig.PrivValidatorListenAddr = "unix://" + tmpfile

	dialer := privval.DialUnixFn(tmpfile)
	dialerEndpoint := privval.NewSignerDialerEndpoint(
		log.TestingLogger(),
		dialer,
	)
	privval.SignerDialerEndpointTimeoutReadWrite(100 * time.Millisecond)(dialerEndpoint)

	pvsc := privval.NewSignerServer(
		dialerEndpoint,
		config.ChainID(),
		types.NewMockPV(),
	)

	go func() {
		err := pvsc.Start()
		require.NoError(t, err)
	}()
	defer pvsc.Stop() //nolint:errcheck // ignore for tests

	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)
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
	config := cfg.ResetTestRoot("node_create_proposal")
	defer os.RemoveAll(config.RootDir)
	cc := proxy.NewLocalClientCreator(kvstore.NewApplication())
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	logger := log.TestingLogger()

	const height int64 = 1
	state, stateDB, privVals := state(1, height)
	stateStore := sm.NewStore(stateDB)
	maxBytes := 16384
	const partSize uint32 = 256
	maxEvidenceBytes := int64(maxBytes / 2)
	state.ConsensusParams.Block.MaxBytes = int64(maxBytes)
	state.ConsensusParams.Evidence.MaxBytes = maxEvidenceBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	// Make Mempool
	mempool := mempl.NewCListMempool(
		config.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
		mempl.WithMetrics(mempl.NopMetrics()),
		mempl.WithPreCheck(sm.TxPreCheck(state)),
		mempl.WithPostCheck(sm.TxPostCheck(state)),
	)
	mempool.SetLogger(logger)

	// Make EvidencePool
	evidenceDB := dbm.NewMemDB()
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	evidencePool, err := evidence.NewPool(logger, evidenceDB, stateStore, blockStore)
	require.NoError(t, err)

	// fill the evidence pool with more evidence
	// than can fit in a block
	var currentBytes int64 = 0
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
		err := mempool.CheckTx(tx, nil, mempl.TxInfo{})
		assert.NoError(t, err)
	}

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		mempool,
		evidencePool,
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
	config := cfg.ResetTestRoot("node_create_proposal")
	defer os.RemoveAll(config.RootDir)
	cc := proxy.NewLocalClientCreator(kvstore.NewApplication())
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	logger := log.TestingLogger()

	const height int64 = 1
	state, stateDB, _ := state(1, height)
	stateStore := sm.NewStore(stateDB)
	const maxBytes int64 = 16384
	const partSize uint32 = 256
	state.ConsensusParams.Block.MaxBytes = maxBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	// Make Mempool
	mempool := mempl.NewCListMempool(
		config.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
		mempl.WithMetrics(mempl.NopMetrics()),
		mempl.WithPreCheck(sm.TxPreCheck(state)),
		mempl.WithPostCheck(sm.TxPostCheck(state)),
	)
	mempool.SetLogger(logger)

	// fill the mempool with one txs just below the maximum size
	txLength := int(types.MaxDataBytesNoEvidence(maxBytes, 1))
	tx := tmrand.Bytes(txLength - 4) // to account for the varint
	err = mempool.CheckTx(tx, nil, mempl.TxInfo{})
	assert.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		mempool,
		sm.EmptyEvidencePool{},
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
	config := cfg.ResetTestRoot("node_create_proposal")
	defer os.RemoveAll(config.RootDir)
	cc := proxy.NewLocalClientCreator(kvstore.NewApplication())
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	logger := log.TestingLogger()

	state, stateDB, _ := state(types.MaxVotesCount, int64(1))
	stateStore := sm.NewStore(stateDB)
	const maxBytes int64 = 1024 * 1024 * 2
	state.ConsensusParams.Block.MaxBytes = maxBytes
	proposerAddr, _ := state.Validators.GetByIndex(0)

	// Make Mempool
	mempool := mempl.NewCListMempool(
		config.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
		mempl.WithMetrics(mempl.NopMetrics()),
		mempl.WithPreCheck(sm.TxPreCheck(state)),
		mempl.WithPostCheck(sm.TxPostCheck(state)),
	)
	mempool.SetLogger(logger)

	// fill the mempool with one txs just below the maximum size
	txLength := int(types.MaxDataBytesNoEvidence(maxBytes, types.MaxVotesCount))
	tx := tmrand.Bytes(txLength - 6) // to account for the varint
	err = mempool.CheckTx(tx, nil, mempl.TxInfo{})
	assert.NoError(t, err)
	// now produce more txs than what a normal block can hold with 10 smaller txs
	// At the end of the test, only the single big tx should be added
	for i := 0; i < 10; i++ {
		tx := tmrand.Bytes(10)
		err = mempool.CheckTx(tx, nil, mempl.TxInfo{})
		assert.NoError(t, err)
	}

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		mempool,
		sm.EmptyEvidencePool{},
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

func TestNodeNewNodeCustomReactors(t *testing.T) {
	config := cfg.ResetTestRoot("node_new_node_custom_reactors_test")
	defer os.RemoveAll(config.RootDir)

	cr := p2pmock.NewReactor()
	customBlockchainReactor := p2pmock.NewReactor()

	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	require.NoError(t, err)
	pval, err := privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	require.NoError(t, err)

	appClient, closer := proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir())
	t.Cleanup(func() { closer.Close() })
	n, err := NewNode(config,
		pval,
		nodeKey,
		appClient,
		DefaultGenesisDocProviderFunc(config),
		DefaultDBProvider,
		DefaultMetricsProvider(config.Instrumentation),
		log.TestingLogger(),
		CustomReactors(map[string]p2p.Reactor{"FOO": cr, "BLOCKCHAIN": customBlockchainReactor}),
	)
	require.NoError(t, err)

	err = n.Start()
	require.NoError(t, err)
	defer n.Stop() //nolint:errcheck // ignore for tests

	assert.True(t, cr.IsRunning())
	assert.Equal(t, cr, n.Switch().Reactor("FOO"))

	assert.True(t, customBlockchainReactor.IsRunning())
	assert.Equal(t, customBlockchainReactor, n.Switch().Reactor("BLOCKCHAIN"))
}

func TestNodeNewSeedNode(t *testing.T) {
	config := cfg.ResetTestRoot("node_new_node_custom_reactors_test")
	config.Mode = cfg.ModeSeed
	defer os.RemoveAll(config.RootDir)

	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	require.NoError(t, err)

	n, err := NewSeedNode(config,
		nodeKey,
		DefaultGenesisDocProviderFunc(config),
		log.TestingLogger(),
	)
	require.NoError(t, err)

	err = n.Start()
	require.NoError(t, err)

	assert.True(t, n.pexReactor.IsRunning())
}

func state(nVals int, height int64) (sm.State, dbm.DB, []types.PrivValidator) {
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
	stateStore := sm.NewStore(stateDB)
	if err := stateStore.Save(s); err != nil {
		panic(err)
	}

	for i := 1; i < int(height); i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()
		if err := stateStore.Save(s); err != nil {
			panic(err)
		}
	}
	return s, stateDB, privVals
}
