package statesync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	proxymocks "github.com/tendermint/tendermint/proxy/mocks"
	"github.com/tendermint/tendermint/statesync/mocks"
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

	peerUpdates *p2p.PeerUpdates
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
		peerUpdates:       p2p.NewPeerUpdates(make(chan p2p.PeerUpdate), int(chBuf)),
		conn:              conn,
		connQuery:         connQuery,
		stateProvider:     stateProvider,
	}

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

	rts.reactor = NewReactor(
		log.NewNopLogger(),
		conn,
		connQuery,
		rts.snapshotChannel,
		rts.chunkChannel,
		rts.peerUpdates,
		"",
	)

	rts.syncer = newSyncer(
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
