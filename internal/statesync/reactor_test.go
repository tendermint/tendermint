package statesync

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	// "github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/statesync/mocks"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light/provider"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	proxymocks "github.com/tendermint/tendermint/proxy/mocks"
	smmocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

type reactorTestSuite struct {
	reactor *Reactor
	syncer  *syncer

	conn          *proxymocks.AppConnSnapshot
	connQuery     *proxymocks.AppConnQuery
	stateProvider *mocks.StateProvider

	snapshotChannel   *p2p.Channel
	snapshotInCh      chan p2p.Envelope
	snapshotOutCh     chan p2p.Envelope
	snapshotPeerErrCh chan p2p.PeerError

	chunkChannel   *p2p.Channel
	chunkInCh      chan p2p.Envelope
	chunkOutCh     chan p2p.Envelope
	chunkPeerErrCh chan p2p.PeerError

	blockChannel   *p2p.Channel
	blockInCh      chan p2p.Envelope
	blockOutCh     chan p2p.Envelope
	blockPeerErrCh chan p2p.PeerError

	peerUpdateCh chan p2p.PeerUpdate
	peerUpdates  *p2p.PeerUpdates

	stateStore *smmocks.Store
	blockStore *store.BlockStore
}

func setup(
	t *testing.T,
	conn *proxymocks.AppConnSnapshot,
	connQuery *proxymocks.AppConnQuery,
	stateProvider *mocks.StateProvider,
	chBuf uint,
) *reactorTestSuite {
	t.Helper()

	if conn == nil {
		conn = &proxymocks.AppConnSnapshot{}
	}
	if connQuery == nil {
		connQuery = &proxymocks.AppConnQuery{}
	}
	if stateProvider == nil {
		stateProvider = &mocks.StateProvider{}
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
		conn:              conn,
		connQuery:         connQuery,
		stateProvider:     stateProvider,
	}

	rts.peerUpdateCh = make(chan p2p.PeerUpdate, chBuf)
	rts.peerUpdates = p2p.NewPeerUpdates(rts.peerUpdateCh, int(chBuf))

	rts.snapshotChannel = p2p.NewChannel(
		SnapshotChannel,
		new(ssproto.Message),
		rts.snapshotInCh,
		rts.snapshotOutCh,
		rts.snapshotPeerErrCh,
	)

	rts.chunkChannel = p2p.NewChannel(
		ChunkChannel,
		new(ssproto.Message),
		rts.chunkInCh,
		rts.chunkOutCh,
		rts.chunkPeerErrCh,
	)

	rts.blockChannel = p2p.NewChannel(
		LightBlockChannel,
		new(ssproto.Message),
		rts.blockInCh,
		rts.blockOutCh,
		rts.blockPeerErrCh,
	)

	rts.stateStore = &smmocks.Store{}
	rts.blockStore = store.NewBlockStore(dbm.NewMemDB())

	cfg := config.DefaultStateSyncConfig()

	rts.reactor = NewReactor(
		*cfg,
		log.TestingLogger(),
		conn,
		connQuery,
		rts.snapshotChannel,
		rts.chunkChannel,
		rts.blockChannel,
		rts.peerUpdates,
		rts.stateStore,
		rts.blockStore,
		"",
	)

	// override the dispatcher with one with a shorter timeout
	rts.reactor.dispatcher = newDispatcher(rts.blockChannel.Out, 1*time.Second)

	rts.syncer = newSyncer(
		*cfg,
		log.NewNopLogger(),
		conn,
		connQuery,
		stateProvider,
		rts.snapshotOutCh,
		rts.chunkOutCh,
		"",
	)

	require.NoError(t, rts.reactor.Start())
	require.True(t, rts.reactor.IsRunning())

	t.Cleanup(func() {
		require.NoError(t, rts.reactor.Stop())
		require.False(t, rts.reactor.IsRunning())
	})

	return rts
}

func TestReactor_ChunkRequest_InvalidRequest(t *testing.T) {
	rts := setup(t, nil, nil, nil, 2)

	rts.chunkInCh <- p2p.Envelope{
		From:    p2p.NodeID("aa"),
		Message: &ssproto.SnapshotsRequest{},
	}

	response := <-rts.chunkPeerErrCh
	require.Error(t, response.Err)
	require.Empty(t, rts.chunkOutCh)
	require.Contains(t, response.Err.Error(), "received unknown message")
	require.Equal(t, p2p.NodeID("aa"), response.NodeID)
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

	for name, tc := range testcases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			// mock ABCI connection to return local snapshots
			conn := &proxymocks.AppConnSnapshot{}
			conn.On("LoadSnapshotChunkSync", context.Background(), abci.RequestLoadSnapshotChunk{
				Height: tc.request.Height,
				Format: tc.request.Format,
				Chunk:  tc.request.Index,
			}).Return(&abci.ResponseLoadSnapshotChunk{Chunk: tc.chunk}, nil)

			rts := setup(t, conn, nil, nil, 2)

			rts.chunkInCh <- p2p.Envelope{
				From:    p2p.NodeID("aa"),
				Message: tc.request,
			}

			response := <-rts.chunkOutCh
			require.Equal(t, tc.expectResponse, response.Message)
			require.Empty(t, rts.chunkOutCh)

			conn.AssertExpectations(t)
		})
	}
}

func TestReactor_SnapshotsRequest_InvalidRequest(t *testing.T) {
	rts := setup(t, nil, nil, nil, 2)

	rts.snapshotInCh <- p2p.Envelope{
		From:    p2p.NodeID("aa"),
		Message: &ssproto.ChunkRequest{},
	}

	response := <-rts.snapshotPeerErrCh
	require.Error(t, response.Err)
	require.Empty(t, rts.snapshotOutCh)
	require.Contains(t, response.Err.Error(), "received unknown message")
	require.Equal(t, p2p.NodeID("aa"), response.NodeID)
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

	for name, tc := range testcases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			// mock ABCI connection to return local snapshots
			conn := &proxymocks.AppConnSnapshot{}
			conn.On("ListSnapshotsSync", context.Background(), abci.RequestListSnapshots{}).Return(&abci.ResponseListSnapshots{
				Snapshots: tc.snapshots,
			}, nil)

			rts := setup(t, conn, nil, nil, 100)

			rts.snapshotInCh <- p2p.Envelope{
				From:    p2p.NodeID("aa"),
				Message: &ssproto.SnapshotsRequest{},
			}

			if len(tc.expectResponses) > 0 {
				retryUntil(t, func() bool { return len(rts.snapshotOutCh) == len(tc.expectResponses) }, time.Second)
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
	rts := setup(t, nil, nil, nil, 2)

	var height int64 = 10
	h := factory.MakeRandomHeader()
	h.Height = height
	blockID := factory.MakeBlockIDWithHash(h.Hash())
	vals, pv := factory.RandValidatorSet(1, 10)
	vote, err := factory.MakeVote(pv[0], h.ChainID, 0, h.Height, 0, 2,
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
		From: p2p.NodeID("aa"),
		Message: &ssproto.LightBlockRequest{
			Height: 10,
		},
	}
	require.Empty(t, rts.blockPeerErrCh)

	select {
	case response := <-rts.blockOutCh:
		require.Equal(t, p2p.NodeID("aa"), response.To)
		res, ok := response.Message.(*ssproto.LightBlockResponse)
		require.True(t, ok)
		receivedLB, err := types.LightBlockFromProto(res.LightBlock)
		require.NoError(t, err)
		require.Equal(t, lb, receivedLB)
	case <-time.After(1 * time.Second):
		t.Fatal("expected light block response")
	}
}

func TestReactor_Dispatcher(t *testing.T) {
	rts := setup(t, nil, nil, nil, 2)
	rts.peerUpdateCh <- p2p.PeerUpdate{
		NodeID: p2p.NodeID("aa"),
		Status: p2p.PeerStatusUp,
	}
	rts.peerUpdateCh <- p2p.PeerUpdate{
		NodeID: p2p.NodeID("bb"),
		Status: p2p.PeerStatusUp,
	}

	closeCh := make(chan struct{})
	defer close(closeCh)

	chain := buildLightBlockChain(t, 1, 10, time.Now())
	go handleLightBlockRequests(t, chain, rts.blockOutCh, rts.blockInCh, closeCh, 0)

	dispatcher := rts.reactor.Dispatcher()
	providers := dispatcher.Providers(factory.DefaultTestChainID, 5*time.Second)
	require.Len(t, providers, 2)

	wg := sync.WaitGroup{}

	for _, p := range providers {
		wg.Add(1)
		go func(t *testing.T, p provider.Provider) {
			defer wg.Done()
			for height := 2; height < 10; height++ {
				lb, err := p.LightBlock(context.Background(), int64(height))
				require.NoError(t, err)
				require.NotNil(t, lb)
				require.Equal(t, height, int(lb.Height))
			}
		}(t, p)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { wg.Wait(); cancel() }()

	select {
	case <-time.After(time.Second):
		// not all of the requests to the dispatcher were responded to
		// within the timeout
		t.Fail()
	case <-ctx.Done():
	}
}

func TestReactor_Backfill(t *testing.T) {
	// test backfill algorithm with varying failure rates [0, 10]
	failureRates := []int{0, 3, 9}
	for _, failureRate := range failureRates {
		failureRate := failureRate
		t.Run(fmt.Sprintf("failure rate: %d", failureRate), func(t *testing.T) {
			// t.Cleanup(leaktest.Check(t))
			rts := setup(t, nil, nil, nil, 21)

			var (
				startHeight int64 = 20
				stopHeight  int64 = 10
				stopTime          = time.Date(2020, 1, 1, 0, 100, 0, 0, time.UTC)
			)

			peers := []string{"a", "b", "c", "d"}
			for _, peer := range peers {
				rts.peerUpdateCh <- p2p.PeerUpdate{
					NodeID: p2p.NodeID(peer),
					Status: p2p.PeerStatusUp,
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

			chain := buildLightBlockChain(t, stopHeight-1, startHeight+1, stopTime)

			closeCh := make(chan struct{})
			defer close(closeCh)
			go handleLightBlockRequests(t, chain, rts.blockOutCh,
				rts.blockInCh, closeCh, failureRate)

			err := rts.reactor.backfill(
				context.Background(),
				factory.DefaultTestChainID,
				startHeight,
				stopHeight,
				factory.MakeBlockIDWithHash(chain[startHeight].Header.Hash()),
				stopTime,
			)
			if failureRate > 5 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				for height := startHeight; height <= stopHeight; height++ {
					blockMeta := rts.blockStore.LoadBlockMeta(height)
					require.NotNil(t, blockMeta)
				}

				require.Nil(t, rts.blockStore.LoadBlockMeta(stopHeight-1))
				require.Nil(t, rts.blockStore.LoadBlockMeta(startHeight+1))
			}
		})
	}
}

// retryUntil will continue to evaluate fn and will return successfully when true
// or fail when the timeout is reached.
func retryUntil(t *testing.T, fn func() bool, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		if fn() {
			return
		}

		require.NoError(t, ctx.Err())
	}
}

func handleLightBlockRequests(t *testing.T,
	chain map[int64]*types.LightBlock,
	receiving chan p2p.Envelope,
	sending chan p2p.Envelope,
	close chan struct{},
	failureRate int) {
	requests := 0
	for {
		select {
		case envelope := <-receiving:
			if msg, ok := envelope.Message.(*ssproto.LightBlockRequest); ok {
				if requests%10 >= failureRate {
					lb, err := chain[int64(msg.Height)].ToProto()
					require.NoError(t, err)
					sending <- p2p.Envelope{
						From: envelope.To,
						Message: &ssproto.LightBlockResponse{
							LightBlock: lb,
						},
					}
				} else {
					switch rand.Intn(3) {
					case 0: // send a different block
						differntLB, err := mockLB(t, int64(msg.Height), factory.DefaultTestTime, factory.MakeBlockID()).ToProto()
						require.NoError(t, err)
						sending <- p2p.Envelope{
							From: envelope.To,
							Message: &ssproto.LightBlockResponse{
								LightBlock: differntLB,
							},
						}
					case 1: // send nil block i.e. pretend we don't have it
						sending <- p2p.Envelope{
							From: envelope.To,
							Message: &ssproto.LightBlockResponse{
								LightBlock: nil,
							},
						}
					case 2: // don't do anything
					}
				}
			}
		case <-close:
			return
		}
		requests++
	}
}

func buildLightBlockChain(t *testing.T, fromHeight, toHeight int64, startTime time.Time) map[int64]*types.LightBlock {
	chain := make(map[int64]*types.LightBlock, toHeight-fromHeight)
	lastBlockID := factory.MakeBlockID()
	blockTime := startTime.Add(-5 * time.Minute)
	for height := fromHeight; height < toHeight; height++ {
		chain[height] = mockLB(t, height, blockTime, lastBlockID)
		lastBlockID = factory.MakeBlockIDWithHash(chain[height].Header.Hash())
		blockTime = blockTime.Add(1 * time.Minute)
	}
	return chain
}

func mockLB(t *testing.T, height int64, time time.Time,
	lastBlockID types.BlockID) *types.LightBlock {
	header, err := factory.MakeHeader(&types.Header{
		Height:      height,
		LastBlockID: lastBlockID,
		Time:        time,
	})
	require.NoError(t, err)
	vals, pv := factory.RandValidatorSet(3, 10)
	header.ValidatorsHash = vals.Hash()
	lastBlockID = factory.MakeBlockIDWithHash(header.Hash())
	voteSet := types.NewVoteSet(factory.DefaultTestChainID, height, 0, tmproto.PrecommitType, vals)
	commit, err := factory.MakeCommit(lastBlockID, height, 0, voteSet, pv, time)
	require.NoError(t, err)
	return &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: header,
			Commit: commit,
		},
		ValidatorSet: vals,
	}
}
