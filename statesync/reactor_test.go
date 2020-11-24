package statesync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	proxymocks "github.com/tendermint/tendermint/proxy/mocks"
)

type ReactorTestSuite struct {
	suite.Suite

	ctx            context.Context
	cancel         context.CancelFunc
	reactor        *Reactor
	snapshotChShim *p2p.ChannelShim
	chunkChShim    *p2p.ChannelShim
	peerUpdateCh   chan p2p.PeerUpdate
}

func (rts *ReactorTestSuite) SetupTest() {
	conn := &proxymocks.AppConnSnapshot{}
	rts.snapshotChShim = p2p.NewChannelShim(GetChannelShims()[p2p.ChannelID(SnapshotChannel)], 100)
	rts.chunkChShim = p2p.NewChannelShim(GetChannelShims()[p2p.ChannelID(ChunkChannel)], 100)
	rts.peerUpdateCh = make(chan p2p.PeerUpdate, 100)

	rts.reactor = NewReactor(
		log.NewNopLogger(),
		nil,
		conn,
		nil,
		rts.snapshotChShim.Channel,
		rts.chunkChShim.Channel,
		rts.peerUpdateCh,
		"",
	)

	// start reactor process
	rts.ctx, rts.cancel = context.WithCancel(context.Background())
	go rts.reactor.Run(rts.ctx)
}

// TearDownTest cleans up the curret test network after _each_ test.
func (rts *ReactorTestSuite) TearDownTest() {
	rts.cancel()
}

func TestReactorTestSuite(t *testing.T) {
	suite.Run(t, new(ReactorTestSuite))
}

func (rts *ReactorTestSuite) TestReactor_ChunkRequest_InvalidRequest() {
	rts.chunkChShim.InCh <- p2p.Envelope{
		From:    p2p.PeerID{0xAA},
		Message: &ssproto.SnapshotsRequest{},
	}

	response := <-rts.chunkChShim.PeerErrCh
	rts.Require().Error(response.Err)
	rts.Require().Contains(response.Err.Error(), "received unknown message")
	rts.Require().Equal(p2p.PeerID{0xAA}, response.PeerID)
}

func (rts *ReactorTestSuite) TestReactor_ChunkRequest() {
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

		rts.Run(name, func() {
			// mock ABCI connection to return local snapshots
			conn := &proxymocks.AppConnSnapshot{}
			conn.On("LoadSnapshotChunkSync", abci.RequestLoadSnapshotChunk{
				Height: tc.request.Height,
				Format: tc.request.Format,
				Chunk:  tc.request.Index,
			}).Return(&abci.ResponseLoadSnapshotChunk{Chunk: tc.chunk}, nil)

			// override the default connection with the mocked version
			rts.reactor.setConn(conn)

			rts.chunkChShim.InCh <- p2p.Envelope{
				From:    p2p.PeerID{0xAA},
				Message: tc.request,
			}

			response := <-rts.chunkChShim.OutCh
			rts.Require().Equal(tc.expectResponse, response.Message)

			conn.AssertExpectations(rts.T())
		})
	}
}

func (rts *ReactorTestSuite) TestReactor_SnapshotsRequest() {
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

		rts.Run(name, func() {
			// mock ABCI connection to return local snapshots
			conn := &proxymocks.AppConnSnapshot{}
			conn.On("ListSnapshotsSync", abci.RequestListSnapshots{}).Return(&abci.ResponseListSnapshots{
				Snapshots: tc.snapshots,
			}, nil)

			// override the default connection with the mocked version
			rts.reactor.setConn(conn)

			rts.snapshotChShim.InCh <- p2p.Envelope{
				From:    p2p.PeerID{0xAA},
				Message: &ssproto.SnapshotsRequest{},
			}

			if len(tc.expectResponses) > 0 {
				retryUntil(rts.T(), func() bool { return len(rts.snapshotChShim.OutCh) == len(tc.expectResponses) }, time.Second)
			}

			responses := make([]*ssproto.SnapshotsResponse, len(tc.expectResponses))
			for i := 0; i < len(tc.expectResponses); i++ {
				e := <-rts.snapshotChShim.OutCh
				responses[i] = e.Message.(*ssproto.SnapshotsResponse)
			}

			rts.Require().Equal(tc.expectResponses, responses)
		})
	}
}

func (rts *ReactorTestSuite) TestReactor_SnapshotsRequest_InvalidRequest() {
	rts.snapshotChShim.InCh <- p2p.Envelope{
		From:    p2p.PeerID{0xAA},
		Message: &ssproto.ChunkRequest{},
	}

	response := <-rts.snapshotChShim.PeerErrCh
	rts.Require().Error(response.Err)
	rts.Require().Contains(response.Err.Error(), "received unknown message")
	rts.Require().Equal(p2p.PeerID{0xAA}, response.PeerID)
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
