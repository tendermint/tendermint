package statesync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/p2p"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"
	proxymocks "github.com/tendermint/tendermint/proxy/mocks"
)

func TestReactor_AddPeer(t *testing.T) {
	peer := &p2pmocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", SnapshotChannel, mock.Anything).Run(func(args mock.Arguments) {
		msg, err := decodeMsg(args[1].([]byte))
		require.NoError(t, err)
		assert.Equal(t, &snapshotsRequestMessage{}, msg)
	}).Return(true)

	r := NewReactor(nil)
	r.AddPeer(peer)
	peer.AssertExpectations(t)
}

func TestReactor_Receive_SnapshotRequestMessage(t *testing.T) {
	testcases := map[string]struct {
		snapshots       []*abci.Snapshot
		expectResponses []*snapshotsResponseMessage
	}{
		"no snapshots": {nil, []*snapshotsResponseMessage{}},
		">10 unordered snapshots": {
			[]*abci.Snapshot{
				{Height: 1, Format: 2, ChunkHashes: [][]byte{{1, 2}}, Metadata: []byte{1}},
				{Height: 2, Format: 2, ChunkHashes: [][]byte{{2, 2}}, Metadata: []byte{2}},
				{Height: 3, Format: 2, ChunkHashes: [][]byte{{3, 2}}, Metadata: []byte{3}},
				{Height: 1, Format: 1, ChunkHashes: [][]byte{{1, 1}}, Metadata: []byte{4}},
				{Height: 2, Format: 1, ChunkHashes: [][]byte{{2, 1}}, Metadata: []byte{5}},
				{Height: 3, Format: 1, ChunkHashes: [][]byte{{3, 1}}, Metadata: []byte{6}},
				{Height: 1, Format: 4, ChunkHashes: [][]byte{{1, 4}}, Metadata: []byte{7}},
				{Height: 2, Format: 4, ChunkHashes: [][]byte{{2, 4}}, Metadata: []byte{8}},
				{Height: 3, Format: 4, ChunkHashes: [][]byte{{3, 4}}, Metadata: []byte{9}},
				{Height: 1, Format: 3, ChunkHashes: [][]byte{{1, 3}}, Metadata: []byte{10}},
				{Height: 2, Format: 3, ChunkHashes: [][]byte{{2, 3}}, Metadata: []byte{11}},
				{Height: 3, Format: 3, ChunkHashes: [][]byte{{3, 3}}, Metadata: []byte{12}},
			},
			[]*snapshotsResponseMessage{
				{Height: 3, Format: 4, ChunkHashes: [][]byte{{3, 4}}, Metadata: []byte{9}},
				{Height: 3, Format: 3, ChunkHashes: [][]byte{{3, 3}}, Metadata: []byte{12}},
				{Height: 3, Format: 2, ChunkHashes: [][]byte{{3, 2}}, Metadata: []byte{3}},
				{Height: 3, Format: 1, ChunkHashes: [][]byte{{3, 1}}, Metadata: []byte{6}},
				{Height: 2, Format: 4, ChunkHashes: [][]byte{{2, 4}}, Metadata: []byte{8}},
				{Height: 2, Format: 3, ChunkHashes: [][]byte{{2, 3}}, Metadata: []byte{11}},
				{Height: 2, Format: 2, ChunkHashes: [][]byte{{2, 2}}, Metadata: []byte{2}},
				{Height: 2, Format: 1, ChunkHashes: [][]byte{{2, 1}}, Metadata: []byte{5}},
				{Height: 1, Format: 4, ChunkHashes: [][]byte{{1, 4}}, Metadata: []byte{7}},
				{Height: 1, Format: 3, ChunkHashes: [][]byte{{1, 3}}, Metadata: []byte{10}},
			},
		},
	}

	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Mock ABCI connection to return local snapshots
			conn := &proxymocks.AppConnSnapshot{}
			conn.On("ListSnapshotsSync", abci.RequestListSnapshots{}).Return(&abci.ResponseListSnapshots{
				Snapshots: tc.snapshots,
			}, nil)

			// Mock peer to catch responses and store them in a slice
			responses := []*snapshotsResponseMessage{}
			peer := &p2pmocks.Peer{}
			if len(tc.expectResponses) > 0 {
				peer.On("Send", SnapshotChannel, mock.Anything).Run(func(args mock.Arguments) {
					msg, err := decodeMsg(args[1].([]byte))
					require.NoError(t, err)
					responses = append(responses, msg.(*snapshotsResponseMessage))
				}).Return(true)
			}

			// Start a reactor and send a SnapshotsRequestMessage, then wait for and check responses
			r := NewReactor(conn)
			err := r.Start()
			require.NoError(t, err)
			defer r.Stop()

			r.Receive(SnapshotChannel, peer, cdc.MustMarshalBinaryBare(&snapshotsRequestMessage{}))
			time.Sleep(100 * time.Millisecond)
			assert.Equal(t, tc.expectResponses, responses)

			conn.AssertExpectations(t)
			peer.AssertExpectations(t)
		})
	}
}
