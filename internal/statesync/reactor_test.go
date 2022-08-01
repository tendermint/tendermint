package statesync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	clientmocks "github.com/tendermint/tendermint/abci/client/mocks"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/proxy"
	smmocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/internal/statesync/mocks"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light/provider"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var m = PrometheusMetrics(config.TestConfig().Instrumentation.Namespace)

const testAppVersion = 9

type reactorTestSuite struct {
	reactor *Reactor
	syncer  *syncer

	conn          *clientmocks.Client
	stateProvider *mocks.StateProvider

	snapshotChannel   p2p.Channel
	snapshotInCh      chan p2p.Envelope
	snapshotOutCh     chan p2p.Envelope
	snapshotPeerErrCh chan p2p.PeerError

	chunkChannel   p2p.Channel
	chunkInCh      chan p2p.Envelope
	chunkOutCh     chan p2p.Envelope
	chunkPeerErrCh chan p2p.PeerError

	blockChannel   p2p.Channel
	blockInCh      chan p2p.Envelope
	blockOutCh     chan p2p.Envelope
	blockPeerErrCh chan p2p.PeerError

	paramsChannel   p2p.Channel
	paramsInCh      chan p2p.Envelope
	paramsOutCh     chan p2p.Envelope
	paramsPeerErrCh chan p2p.PeerError

	peerUpdateCh chan p2p.PeerUpdate
	peerUpdates  *p2p.PeerUpdates

	stateStore *smmocks.Store
	blockStore *store.BlockStore
}

func setup(
	ctx context.Context,
	t *testing.T,
	conn *clientmocks.Client,
	stateProvider *mocks.StateProvider,
	chBuf uint,
) *reactorTestSuite {
	t.Helper()

	if conn == nil {
		conn = &clientmocks.Client{}
	}

	rts := &reactorTestSuite{
		snapshotInCh:      make(chan p2p.Envelope, chBuf),
		snapshotOutCh:     make(chan p2p.Envelope, chBuf),
		snapshotPeerErrCh: make(chan p2p.PeerError, chBuf),
		chunkInCh:         make(chan p2p.Envelope, chBuf),
		chunkOutCh:        make(chan p2p.Envelope, chBuf),
		chunkPeerErrCh:    make(chan p2p.PeerError, chBuf),
		blockInCh:         make(chan p2p.Envelope, chBuf),
		blockOutCh:        make(chan p2p.Envelope, chBuf),
		blockPeerErrCh:    make(chan p2p.PeerError, chBuf),
		paramsInCh:        make(chan p2p.Envelope, chBuf),
		paramsOutCh:       make(chan p2p.Envelope, chBuf),
		paramsPeerErrCh:   make(chan p2p.PeerError, chBuf),
		conn:              conn,
		stateProvider:     stateProvider,
	}

	rts.peerUpdateCh = make(chan p2p.PeerUpdate, chBuf)
	rts.peerUpdates = p2p.NewPeerUpdates(rts.peerUpdateCh, int(chBuf))

	rts.snapshotChannel = p2p.NewChannel(
		SnapshotChannel,
		"snapshot",
		rts.snapshotInCh,
		rts.snapshotOutCh,
		rts.snapshotPeerErrCh,
	)

	rts.chunkChannel = p2p.NewChannel(
		ChunkChannel,
		"chunk",
		rts.chunkInCh,
		rts.chunkOutCh,
		rts.chunkPeerErrCh,
	)

	rts.blockChannel = p2p.NewChannel(
		LightBlockChannel,
		"lightblock",
		rts.blockInCh,
		rts.blockOutCh,
		rts.blockPeerErrCh,
	)

	rts.paramsChannel = p2p.NewChannel(
		ParamsChannel,
		"params",
		rts.paramsInCh,
		rts.paramsOutCh,
		rts.paramsPeerErrCh,
	)

	rts.stateStore = &smmocks.Store{}
	rts.blockStore = store.NewBlockStore(dbm.NewMemDB())

	cfg := config.DefaultStateSyncConfig()

	chCreator := func(ctx context.Context, desc *p2p.ChannelDescriptor) (p2p.Channel, error) {
		switch desc.ID {
		case SnapshotChannel:
			return rts.snapshotChannel, nil
		case ChunkChannel:
			return rts.chunkChannel, nil
		case LightBlockChannel:
			return rts.blockChannel, nil
		case ParamsChannel:
			return rts.paramsChannel, nil
		default:
			return nil, fmt.Errorf("invalid channel; %v", desc.ID)
		}
	}

	logger := log.NewNopLogger()

	rts.reactor = NewReactor(
		factory.DefaultTestChainID,
		1,
		*cfg,
		logger.With("component", "reactor"),
		conn,
		chCreator,
		func(context.Context) *p2p.PeerUpdates { return rts.peerUpdates },
		rts.stateStore,
		rts.blockStore,
		"",
		m,
		nil,   // eventbus can be nil
		nil,   // post-sync-hook
		false, // run Sync during Start()
	)

	rts.syncer = &syncer{
		logger:        logger,
		stateProvider: stateProvider,
		conn:          conn,
		snapshots:     newSnapshotPool(),
		snapshotCh:    rts.snapshotChannel,
		chunkCh:       rts.chunkChannel,
		tempDir:       t.TempDir(),
		fetchers:      cfg.Fetchers,
		retryTimeout:  cfg.ChunkRequestTimeout,
		metrics:       rts.reactor.metrics,
	}

	ctx, cancel := context.WithCancel(ctx)

	require.NoError(t, rts.reactor.Start(ctx))
	require.True(t, rts.reactor.IsRunning())

	t.Cleanup(cancel)
	t.Cleanup(rts.reactor.Wait)
	t.Cleanup(leaktest.Check(t))

	return rts
}

func TestReactor_Sync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const snapshotHeight = 7
	rts := setup(ctx, t, nil, nil, 100)
	chain := buildLightBlockChain(ctx, t, 1, 10, time.Now())
	// app accepts any snapshot
	rts.conn.On("OfferSnapshot", ctx, mock.IsType(&abci.RequestOfferSnapshot{})).
		Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}, nil)

	// app accepts every chunk
	rts.conn.On("ApplySnapshotChunk", ctx, mock.IsType(&abci.RequestApplySnapshotChunk{})).
		Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)

	// app query returns valid state app hash
	rts.conn.On("Info", mock.Anything, &proxy.RequestInfo).Return(&abci.ResponseInfo{
		AppVersion:       testAppVersion,
		LastBlockHeight:  snapshotHeight,
		LastBlockAppHash: chain[snapshotHeight+1].AppHash,
	}, nil)

	// store accepts state and validator sets
	rts.stateStore.On("Bootstrap", mock.AnythingOfType("state.State")).Return(nil)
	rts.stateStore.On("SaveValidatorSets", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"),
		mock.AnythingOfType("*types.ValidatorSet")).Return(nil)

	closeCh := make(chan struct{})
	defer close(closeCh)
	go handleLightBlockRequests(ctx, t, chain, rts.blockOutCh, rts.blockInCh, closeCh, 0)
	go graduallyAddPeers(ctx, t, rts.peerUpdateCh, closeCh, 1*time.Second)
	go handleSnapshotRequests(ctx, t, rts.snapshotOutCh, rts.snapshotInCh, closeCh, []snapshot{
		{
			Height: uint64(snapshotHeight),
			Format: 1,
			Chunks: 1,
		},
	})

	go handleChunkRequests(ctx, t, rts.chunkOutCh, rts.chunkInCh, closeCh, []byte("abc"))

	go handleConsensusParamsRequest(ctx, t, rts.paramsOutCh, rts.paramsInCh, closeCh)

	// update the config to use the p2p provider
	rts.reactor.cfg.UseP2P = true
	rts.reactor.cfg.TrustHeight = 1
	rts.reactor.cfg.TrustHash = fmt.Sprintf("%X", chain[1].Hash())
	rts.reactor.cfg.DiscoveryTime = 1 * time.Second

	// Run state sync
	_, err := rts.reactor.Sync(ctx)
	require.NoError(t, err)
}

func TestReactor_ChunkRequest_InvalidRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rts := setup(ctx, t, nil, nil, 2)

	rts.chunkInCh <- p2p.Envelope{
		From:      types.NodeID("aa"),
		ChannelID: ChunkChannel,
		Message:   &ssproto.SnapshotsRequest{},
	}

	response := <-rts.chunkPeerErrCh
	require.Error(t, response.Err)
	require.Empty(t, rts.chunkOutCh)
	require.Contains(t, response.Err.Error(), "received unknown message")
	require.Equal(t, types.NodeID("aa"), response.NodeID)
}

func TestReactor_ChunkRequest(t *testing.T) {
	testcases := map[string]struct {
		request        *ssproto.ChunkRequest
		chunk          []byte
		expectResponse *ssproto.ChunkResponse
	}{
		"chunk is returned": {
			&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1},
			[]byte{1, 2, 3},
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: []byte{1, 2, 3}},
		},
		"empty chunk is returned, as empty": {
			&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1},
			[]byte{},
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: []byte{}},
		},
		"nil (missing) chunk is returned as missing": {
			&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1},
			nil,
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true},
		},
		"invalid request": {
			&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1},
			nil,
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true},
		},
	}

	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(bctx)
			defer cancel()

			// mock ABCI connection to return local snapshots
			conn := &clientmocks.Client{}
			conn.On("LoadSnapshotChunk", mock.Anything, &abci.RequestLoadSnapshotChunk{
				Height: tc.request.Height,
				Format: tc.request.Format,
				Chunk:  tc.request.Index,
			}).Return(&abci.ResponseLoadSnapshotChunk{Chunk: tc.chunk}, nil)

			rts := setup(ctx, t, conn, nil, 2)

			rts.chunkInCh <- p2p.Envelope{
				From:      types.NodeID("aa"),
				ChannelID: ChunkChannel,
				Message:   tc.request,
			}

			response := <-rts.chunkOutCh
			require.Equal(t, tc.expectResponse, response.Message)
			require.Empty(t, rts.chunkOutCh)

			conn.AssertExpectations(t)
		})
	}
}

func TestReactor_SnapshotsRequest_InvalidRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rts := setup(ctx, t, nil, nil, 2)

	rts.snapshotInCh <- p2p.Envelope{
		From:      types.NodeID("aa"),
		ChannelID: SnapshotChannel,
		Message:   &ssproto.ChunkRequest{},
	}

	response := <-rts.snapshotPeerErrCh
	require.Error(t, response.Err)
	require.Empty(t, rts.snapshotOutCh)
	require.Contains(t, response.Err.Error(), "received unknown message")
	require.Equal(t, types.NodeID("aa"), response.NodeID)
}

func TestReactor_SnapshotsRequest(t *testing.T) {
	testcases := map[string]struct {
		snapshots       []*abci.Snapshot
		expectResponses []*ssproto.SnapshotsResponse
	}{
		"no snapshots": {nil, []*ssproto.SnapshotsResponse{}},
		">10 unordered snapshots": {
			[]*abci.Snapshot{
				{Height: 1, Format: 2, Chunks: 7, Hash: []byte{1, 2}, Metadata: []byte{1}},
				{Height: 2, Format: 2, Chunks: 7, Hash: []byte{2, 2}, Metadata: []byte{2}},
				{Height: 3, Format: 2, Chunks: 7, Hash: []byte{3, 2}, Metadata: []byte{3}},
				{Height: 1, Format: 1, Chunks: 7, Hash: []byte{1, 1}, Metadata: []byte{4}},
				{Height: 2, Format: 1, Chunks: 7, Hash: []byte{2, 1}, Metadata: []byte{5}},
				{Height: 3, Format: 1, Chunks: 7, Hash: []byte{3, 1}, Metadata: []byte{6}},
				{Height: 1, Format: 4, Chunks: 7, Hash: []byte{1, 4}, Metadata: []byte{7}},
				{Height: 2, Format: 4, Chunks: 7, Hash: []byte{2, 4}, Metadata: []byte{8}},
				{Height: 3, Format: 4, Chunks: 7, Hash: []byte{3, 4}, Metadata: []byte{9}},
				{Height: 1, Format: 3, Chunks: 7, Hash: []byte{1, 3}, Metadata: []byte{10}},
				{Height: 2, Format: 3, Chunks: 7, Hash: []byte{2, 3}, Metadata: []byte{11}},
				{Height: 3, Format: 3, Chunks: 7, Hash: []byte{3, 3}, Metadata: []byte{12}},
			},
			[]*ssproto.SnapshotsResponse{
				{Height: 3, Format: 4, Chunks: 7, Hash: []byte{3, 4}, Metadata: []byte{9}},
				{Height: 3, Format: 3, Chunks: 7, Hash: []byte{3, 3}, Metadata: []byte{12}},
				{Height: 3, Format: 2, Chunks: 7, Hash: []byte{3, 2}, Metadata: []byte{3}},
				{Height: 3, Format: 1, Chunks: 7, Hash: []byte{3, 1}, Metadata: []byte{6}},
				{Height: 2, Format: 4, Chunks: 7, Hash: []byte{2, 4}, Metadata: []byte{8}},
				{Height: 2, Format: 3, Chunks: 7, Hash: []byte{2, 3}, Metadata: []byte{11}},
				{Height: 2, Format: 2, Chunks: 7, Hash: []byte{2, 2}, Metadata: []byte{2}},
				{Height: 2, Format: 1, Chunks: 7, Hash: []byte{2, 1}, Metadata: []byte{5}},
				{Height: 1, Format: 4, Chunks: 7, Hash: []byte{1, 4}, Metadata: []byte{7}},
				{Height: 1, Format: 3, Chunks: 7, Hash: []byte{1, 3}, Metadata: []byte{10}},
			},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for name, tc := range testcases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			// mock ABCI connection to return local snapshots
			conn := &clientmocks.Client{}
			conn.On("ListSnapshots", mock.Anything, &abci.RequestListSnapshots{}).Return(&abci.ResponseListSnapshots{
				Snapshots: tc.snapshots,
			}, nil)

			rts := setup(ctx, t, conn, nil, 100)

			rts.snapshotInCh <- p2p.Envelope{
				From:      types.NodeID("aa"),
				ChannelID: SnapshotChannel,
				Message:   &ssproto.SnapshotsRequest{},
			}

			if len(tc.expectResponses) > 0 {
				retryUntil(ctx, t, func() bool { return len(rts.snapshotOutCh) == len(tc.expectResponses) }, time.Second)
			}

			responses := make([]*ssproto.SnapshotsResponse, len(tc.expectResponses))
			for i := 0; i < len(tc.expectResponses); i++ {
				e := <-rts.snapshotOutCh
				responses[i] = e.Message.(*ssproto.SnapshotsResponse)
			}

			require.Equal(t, tc.expectResponses, responses)
			require.Empty(t, rts.snapshotOutCh)
		})
	}
}

func TestReactor_LightBlockResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rts := setup(ctx, t, nil, nil, 2)

	var height int64 = 10
	// generates a random header
	h := factory.MakeHeader(t, &types.Header{})
	h.Height = height
	blockID := factory.MakeBlockIDWithHash(h.Hash())
	vals, pv := factory.ValidatorSet(ctx, t, 1, 10)
	vote, err := factory.MakeVote(ctx, pv[0], h.ChainID, 0, h.Height, 0, 2,
		blockID, factory.DefaultTestTime)
	require.NoError(t, err)

	sh := &types.SignedHeader{
		Header: h,
		Commit: &types.Commit{
			Height:  h.Height,
			BlockID: blockID,
			Signatures: []types.CommitSig{
				vote.CommitSig(),
			},
		},
	}

	lb := &types.LightBlock{
		SignedHeader: sh,
		ValidatorSet: vals,
	}

	require.NoError(t, rts.blockStore.SaveSignedHeader(sh, blockID))

	rts.stateStore.On("LoadValidators", height).Return(vals, nil)

	rts.blockInCh <- p2p.Envelope{
		From:      types.NodeID("aa"),
		ChannelID: LightBlockChannel,
		Message: &ssproto.LightBlockRequest{
			Height: 10,
		},
	}
	require.Empty(t, rts.blockPeerErrCh)

	select {
	case response := <-rts.blockOutCh:
		require.Equal(t, types.NodeID("aa"), response.To)
		res, ok := response.Message.(*ssproto.LightBlockResponse)
		require.True(t, ok)
		receivedLB, err := types.LightBlockFromProto(res.LightBlock)
		require.NoError(t, err)
		require.Equal(t, lb, receivedLB)
	case <-time.After(1 * time.Second):
		t.Fatal("expected light block response")
	}
}

func TestReactor_BlockProviders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rts := setup(ctx, t, nil, nil, 2)
	rts.peerUpdateCh <- p2p.PeerUpdate{
		NodeID: types.NodeID("aa"),
		Status: p2p.PeerStatusUp,
		Channels: p2p.ChannelIDSet{
			SnapshotChannel:   struct{}{},
			ChunkChannel:      struct{}{},
			LightBlockChannel: struct{}{},
			ParamsChannel:     struct{}{},
		},
	}
	rts.peerUpdateCh <- p2p.PeerUpdate{
		NodeID: types.NodeID("bb"),
		Status: p2p.PeerStatusUp,
		Channels: p2p.ChannelIDSet{
			SnapshotChannel:   struct{}{},
			ChunkChannel:      struct{}{},
			LightBlockChannel: struct{}{},
			ParamsChannel:     struct{}{},
		},
	}

	closeCh := make(chan struct{})
	defer close(closeCh)

	chain := buildLightBlockChain(ctx, t, 1, 10, time.Now())
	go handleLightBlockRequests(ctx, t, chain, rts.blockOutCh, rts.blockInCh, closeCh, 0)

	peers := rts.reactor.peers.All()
	require.Len(t, peers, 2)

	providers := make([]provider.Provider, len(peers))
	for idx, peer := range peers {
		providers[idx] = NewBlockProvider(peer, factory.DefaultTestChainID, rts.reactor.dispatcher)
	}

	wg := sync.WaitGroup{}

	for _, p := range providers {
		wg.Add(1)
		go func(t *testing.T, p provider.Provider) {
			defer wg.Done()
			for height := 2; height < 10; height++ {
				lb, err := p.LightBlock(ctx, int64(height))
				require.NoError(t, err)
				require.NotNil(t, lb)
				require.Equal(t, height, int(lb.Height))
			}
		}(t, p)
	}

	go func() { wg.Wait(); cancel() }()

	select {
	case <-time.After(time.Second):
		// not all of the requests to the dispatcher were responded to
		// within the timeout
		t.Fail()
	case <-ctx.Done():
	}

}

func TestReactor_StateProviderP2P(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rts := setup(ctx, t, nil, nil, 2)
	// make syncer non nil else test won't think we are state syncing
	rts.reactor.syncer = rts.syncer
	peerA := types.NodeID(strings.Repeat("a", 2*types.NodeIDByteLength))
	peerB := types.NodeID(strings.Repeat("b", 2*types.NodeIDByteLength))
	rts.peerUpdateCh <- p2p.PeerUpdate{
		NodeID: peerA,
		Status: p2p.PeerStatusUp,
	}
	rts.peerUpdateCh <- p2p.PeerUpdate{
		NodeID: peerB,
		Status: p2p.PeerStatusUp,
	}

	closeCh := make(chan struct{})
	defer close(closeCh)

	chain := buildLightBlockChain(ctx, t, 1, 10, time.Now())
	go handleLightBlockRequests(ctx, t, chain, rts.blockOutCh, rts.blockInCh, closeCh, 0)
	go handleConsensusParamsRequest(ctx, t, rts.paramsOutCh, rts.paramsInCh, closeCh)

	rts.reactor.cfg.UseP2P = true
	rts.reactor.cfg.TrustHeight = 1
	rts.reactor.cfg.TrustHash = fmt.Sprintf("%X", chain[1].Hash())

	for _, p := range []types.NodeID{peerA, peerB} {
		if !rts.reactor.peers.Contains(p) {
			rts.reactor.peers.Append(p)
		}
	}
	require.True(t, rts.reactor.peers.Len() >= 2, "peer network not configured")

	ictx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	rts.reactor.mtx.Lock()
	err := rts.reactor.initStateProvider(ictx, factory.DefaultTestChainID, 1)
	rts.reactor.mtx.Unlock()
	require.NoError(t, err)
	rts.reactor.syncer.stateProvider = rts.reactor.stateProvider

	actx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	appHash, err := rts.reactor.stateProvider.AppHash(actx, 5)
	require.NoError(t, err)
	require.Len(t, appHash, 32)

	state, err := rts.reactor.stateProvider.State(actx, 5)
	require.NoError(t, err)
	require.Equal(t, appHash, state.AppHash)
	require.Equal(t, types.DefaultConsensusParams(), &state.ConsensusParams)

	commit, err := rts.reactor.stateProvider.Commit(actx, 5)
	require.NoError(t, err)
	require.Equal(t, commit.BlockID, state.LastBlockID)

	added, err := rts.reactor.syncer.AddSnapshot(peerA, &snapshot{
		Height: 1, Format: 2, Chunks: 7, Hash: []byte{1, 2}, Metadata: []byte{1},
	})
	require.NoError(t, err)
	require.True(t, added)
}

func TestReactor_Backfill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// test backfill algorithm with varying failure rates [0, 10]
	failureRates := []int{0, 2, 9}
	for _, failureRate := range failureRates {
		failureRate := failureRate
		t.Run(fmt.Sprintf("failure rate: %d", failureRate), func(t *testing.T) {
			if testing.Short() && failureRate > 0 {
				t.Skip("skipping test in short mode")
			}

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			t.Cleanup(leaktest.CheckTimeout(t, 1*time.Minute))
			rts := setup(ctx, t, nil, nil, 21)

			var (
				startHeight int64 = 20
				stopHeight  int64 = 10
				stopTime          = time.Date(2020, 1, 1, 0, 100, 0, 0, time.UTC)
			)

			peers := []string{"a", "b", "c", "d"}
			for _, peer := range peers {
				rts.peerUpdateCh <- p2p.PeerUpdate{
					NodeID: types.NodeID(peer),
					Status: p2p.PeerStatusUp,
					Channels: p2p.ChannelIDSet{
						SnapshotChannel:   struct{}{},
						ChunkChannel:      struct{}{},
						LightBlockChannel: struct{}{},
						ParamsChannel:     struct{}{},
					},
				}
			}

			trackingHeight := startHeight
			rts.stateStore.On("SaveValidatorSets", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"),
				mock.AnythingOfType("*types.ValidatorSet")).Return(func(lh, uh int64, vals *types.ValidatorSet) error {
				require.Equal(t, trackingHeight, lh)
				require.Equal(t, lh, uh)
				require.GreaterOrEqual(t, lh, stopHeight)
				trackingHeight--
				return nil
			})

			chain := buildLightBlockChain(ctx, t, stopHeight-1, startHeight+1, stopTime)

			closeCh := make(chan struct{})
			defer close(closeCh)
			go handleLightBlockRequests(ctx, t, chain, rts.blockOutCh,
				rts.blockInCh, closeCh, failureRate)

			err := rts.reactor.backfill(
				ctx,
				factory.DefaultTestChainID,
				startHeight,
				stopHeight,
				1,
				factory.MakeBlockIDWithHash(chain[startHeight].Header.Hash()),
				stopTime,
			)
			if failureRate > 3 {
				require.Error(t, err)

				require.NotEqual(t, rts.reactor.backfilledBlocks, rts.reactor.backfillBlockTotal)
				require.Equal(t, startHeight-stopHeight+1, rts.reactor.backfillBlockTotal)
			} else {
				require.NoError(t, err)

				for height := startHeight; height <= stopHeight; height++ {
					blockMeta := rts.blockStore.LoadBlockMeta(height)
					require.NotNil(t, blockMeta)
				}

				require.Nil(t, rts.blockStore.LoadBlockMeta(stopHeight-1))
				require.Nil(t, rts.blockStore.LoadBlockMeta(startHeight+1))

				require.Equal(t, startHeight-stopHeight+1, rts.reactor.backfilledBlocks)
				require.Equal(t, startHeight-stopHeight+1, rts.reactor.backfillBlockTotal)
			}
			require.Equal(t, rts.reactor.backfilledBlocks, rts.reactor.BackFilledBlocks())
			require.Equal(t, rts.reactor.backfillBlockTotal, rts.reactor.BackFillBlocksTotal())
		})
	}
}

// retryUntil will continue to evaluate fn and will return successfully when true
// or fail when the timeout is reached.
func retryUntil(ctx context.Context, t *testing.T, fn func() bool, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		if fn() {
			return
		}
		require.NoError(t, ctx.Err())
	}
}

func handleLightBlockRequests(
	ctx context.Context,
	t *testing.T,
	chain map[int64]*types.LightBlock,
	receiving chan p2p.Envelope,
	sending chan p2p.Envelope,
	close chan struct{},
	failureRate int) {
	requests := 0
	errorCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case envelope := <-receiving:
			if msg, ok := envelope.Message.(*ssproto.LightBlockRequest); ok {
				if requests%10 >= failureRate {
					lb, err := chain[int64(msg.Height)].ToProto()
					require.NoError(t, err)
					select {
					case sending <- p2p.Envelope{
						From:      envelope.To,
						ChannelID: LightBlockChannel,
						Message: &ssproto.LightBlockResponse{
							LightBlock: lb,
						},
					}:
					case <-ctx.Done():
						return
					}
				} else {
					switch errorCount % 3 {
					case 0: // send a different block
						vals, pv := factory.ValidatorSet(ctx, t, 3, 10)
						_, _, lb := mockLB(ctx, t, int64(msg.Height), factory.DefaultTestTime, factory.MakeBlockID(), vals, pv)
						differntLB, err := lb.ToProto()
						require.NoError(t, err)
						select {
						case sending <- p2p.Envelope{
							From:      envelope.To,
							ChannelID: LightBlockChannel,
							Message: &ssproto.LightBlockResponse{
								LightBlock: differntLB,
							},
						}:
						case <-ctx.Done():
							return
						}
					case 1: // send nil block i.e. pretend we don't have it
						select {
						case sending <- p2p.Envelope{
							From:      envelope.To,
							ChannelID: LightBlockChannel,
							Message: &ssproto.LightBlockResponse{
								LightBlock: nil,
							},
						}:
						case <-ctx.Done():
							return
						}
					case 2: // don't do anything
					}
					errorCount++
				}
			}
		case <-close:
			return
		}
		requests++
	}
}

func handleConsensusParamsRequest(
	ctx context.Context,
	t *testing.T,
	receiving, sending chan p2p.Envelope,
	closeCh chan struct{},
) {
	t.Helper()
	params := types.DefaultConsensusParams()
	paramsProto := params.ToProto()
	for {
		select {
		case <-ctx.Done():
			return
		case envelope := <-receiving:
			msg, ok := envelope.Message.(*ssproto.ParamsRequest)
			if !ok {
				t.Errorf("message was %T which is not a params request", envelope.Message)
				return
			}
			select {
			case sending <- p2p.Envelope{
				From:      envelope.To,
				ChannelID: ParamsChannel,
				Message: &ssproto.ParamsResponse{
					Height:          msg.Height,
					ConsensusParams: paramsProto,
				},
			}:
			case <-ctx.Done():
				return
			case <-closeCh:
				return
			}

		case <-closeCh:
			return
		}
	}
}

func buildLightBlockChain(ctx context.Context, t *testing.T, fromHeight, toHeight int64, startTime time.Time) map[int64]*types.LightBlock {
	t.Helper()
	chain := make(map[int64]*types.LightBlock, toHeight-fromHeight)
	lastBlockID := factory.MakeBlockID()
	blockTime := startTime.Add(time.Duration(fromHeight-toHeight) * time.Minute)
	vals, pv := factory.ValidatorSet(ctx, t, 3, 10)
	for height := fromHeight; height < toHeight; height++ {
		vals, pv, chain[height] = mockLB(ctx, t, height, blockTime, lastBlockID, vals, pv)
		lastBlockID = factory.MakeBlockIDWithHash(chain[height].Header.Hash())
		blockTime = blockTime.Add(1 * time.Minute)
	}
	return chain
}

func mockLB(ctx context.Context, t *testing.T, height int64, time time.Time, lastBlockID types.BlockID,
	currentVals *types.ValidatorSet, currentPrivVals []types.PrivValidator,
) (*types.ValidatorSet, []types.PrivValidator, *types.LightBlock) {
	t.Helper()
	header := factory.MakeHeader(t, &types.Header{
		Height:      height,
		LastBlockID: lastBlockID,
		Time:        time,
	})
	header.Version.App = testAppVersion

	nextVals, nextPrivVals := factory.ValidatorSet(ctx, t, 3, 10)
	header.ValidatorsHash = currentVals.Hash()
	header.NextValidatorsHash = nextVals.Hash()
	header.ConsensusHash = types.DefaultConsensusParams().HashConsensusParams()
	lastBlockID = factory.MakeBlockIDWithHash(header.Hash())
	voteSet := types.NewExtendedVoteSet(factory.DefaultTestChainID, height, 0, tmproto.PrecommitType, currentVals)
	extCommit, err := factory.MakeExtendedCommit(ctx, lastBlockID, height, 0, voteSet, currentPrivVals, time)
	require.NoError(t, err)
	return nextVals, nextPrivVals, &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: header,
			Commit: extCommit.ToCommit(),
		},
		ValidatorSet: currentVals,
	}
}

// graduallyAddPeers delivers a new randomly-generated peer update on peerUpdateCh once
// per interval, until closeCh is closed. Each peer update is assigned a random node ID.
func graduallyAddPeers(
	ctx context.Context,
	t *testing.T,
	peerUpdateCh chan p2p.PeerUpdate,
	closeCh chan struct{},
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-closeCh:
			return
		case <-ticker.C:
			peerUpdateCh <- p2p.PeerUpdate{
				NodeID: factory.RandomNodeID(t),
				Status: p2p.PeerStatusUp,
				Channels: p2p.ChannelIDSet{
					SnapshotChannel:   struct{}{},
					ChunkChannel:      struct{}{},
					LightBlockChannel: struct{}{},
					ParamsChannel:     struct{}{},
				},
			}
		}
	}
}

func handleSnapshotRequests(
	ctx context.Context,
	t *testing.T,
	receivingCh chan p2p.Envelope,
	sendingCh chan p2p.Envelope,
	closeCh chan struct{},
	snapshots []snapshot,
) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			return
		case <-closeCh:
			return
		case envelope := <-receivingCh:
			_, ok := envelope.Message.(*ssproto.SnapshotsRequest)
			require.True(t, ok)
			for _, snapshot := range snapshots {
				sendingCh <- p2p.Envelope{
					From:      envelope.To,
					ChannelID: SnapshotChannel,
					Message: &ssproto.SnapshotsResponse{
						Height:   snapshot.Height,
						Format:   snapshot.Format,
						Chunks:   snapshot.Chunks,
						Hash:     snapshot.Hash,
						Metadata: snapshot.Metadata,
					},
				}
			}
		}
	}
}

func handleChunkRequests(
	ctx context.Context,
	t *testing.T,
	receivingCh chan p2p.Envelope,
	sendingCh chan p2p.Envelope,
	closeCh chan struct{},
	chunk []byte,
) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			return
		case <-closeCh:
			return
		case envelope := <-receivingCh:
			msg, ok := envelope.Message.(*ssproto.ChunkRequest)
			require.True(t, ok)
			sendingCh <- p2p.Envelope{
				From:      envelope.To,
				ChannelID: ChunkChannel,
				Message: &ssproto.ChunkResponse{
					Height:  msg.Height,
					Format:  msg.Format,
					Index:   msg.Index,
					Chunk:   chunk,
					Missing: false,
				},
			}

		}
	}
}
