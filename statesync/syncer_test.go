package statesync

import (
	"testing"
	time "time"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"
	"github.com/tendermint/tendermint/proxy"
	proxymocks "github.com/tendermint/tendermint/proxy/mocks"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func TestSyncer(t *testing.T) {
	state := sm.State{
		ChainID:                          "chain",
		ConsensusParams:                  types.ConsensusParams{Block: types.DefaultBlockParams()},
		LastHeightConsensusParamsChanged: 1,
	}
	chunks := []*chunk{
		{Height: 1, Format: 1, Index: 0, Body: []byte{1, 1, 0}},
		{Height: 1, Format: 1, Index: 1, Body: []byte{1, 1, 1}},
		{Height: 1, Format: 1, Index: 2, Body: []byte{1, 1, 2}},
	}
	s := &snapshot{Height: 1, Format: 1, ChunkHashes: [][]byte{}}
	for _, c := range chunks {
		s.ChunkHashes = append(s.ChunkHashes, c.Hash())
	}

	connSnapshot := &proxymocks.AppConnSnapshot{}
	connQuery := &proxymocks.AppConnQuery{}
	lc := &mockLightClient{}
	lc.On("VerifyHeaderAtHeight", int64(1), mock.Anything).Return(&types.SignedHeader{
		Header: &types.Header{
			Height: 1,
			Time:   time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Commit: &types.Commit{BlockID: types.BlockID{Hash: []byte("blockid")}},
	}, nil)
	lc.On("VerifyHeaderAtHeight", int64(2), mock.Anything).Return(&types.SignedHeader{
		Header: &types.Header{Height: 2, AppHash: []byte("app_hash"), LastResultsHash: []byte("last_results_hash")},
	}, nil)
	lc.On("VerifyHeaderAtHeight", int64(3), mock.Anything).Return(&types.SignedHeader{
		Header: &types.Header{Height: 3, AppHash: []byte("app_hash_2")},
	}, nil)
	lc.On("TrustedValidatorSet", int64(1)).Return(&types.ValidatorSet{
		Proposer: &types.Validator{Address: []byte("val1")},
	}, int64(1), nil)
	lc.On("TrustedValidatorSet", int64(2)).Return(&types.ValidatorSet{
		Proposer: &types.Validator{Address: []byte("val2")},
	}, int64(2), nil)
	lc.On("TrustedValidatorSet", int64(3)).Return(&types.ValidatorSet{
		Proposer: &types.Validator{Address: []byte("val3")},
	}, int64(3), nil)

	sync := newSyncer(log.NewNopLogger(), connSnapshot, connQuery, lc)

	// Adding a chunk should error when no sync is in progress
	_, err := sync.AddChunk(&chunk{Height: 1, Format: 1, Index: 0, Body: []byte{1}})
	require.Error(t, err)

	// With no snapshots, starting a sync should error
	_, _, err = sync.Sync(state)
	require.Error(t, err)
	assert.Equal(t, errNoSnapshots, err)

	// Adding a couple of peers should trigger snapshot discovery messages
	peerA := &p2pmocks.Peer{}
	peerA.On("ID").Return(p2p.ID("a"))
	peerA.On("Send", SnapshotChannel, cdc.MustMarshalBinaryBare(&snapshotsRequestMessage{})).Return(true)
	sync.AddPeer(peerA)
	peerA.AssertExpectations(t)

	peerB := &p2pmocks.Peer{}
	peerB.On("ID").Return(p2p.ID("b"))
	peerB.On("Send", SnapshotChannel, cdc.MustMarshalBinaryBare(&snapshotsRequestMessage{})).Return(true)
	sync.AddPeer(peerB)
	peerB.AssertExpectations(t)

	// Both peers report back with snapshots. One of them also returns a snapshot we don't want, in
	// format 2, which will be rejected by the ABCI application.
	new, err := sync.AddSnapshot(peerA, s)
	require.NoError(t, err)
	assert.True(t, new)

	new, err = sync.AddSnapshot(peerB, s)
	require.NoError(t, err)
	assert.False(t, new)

	new, err = sync.AddSnapshot(peerB, &snapshot{Height: 2, Format: 2, ChunkHashes: [][]byte{{1}}})
	require.NoError(t, err)
	assert.True(t, new)

	// We start a sync, with peers sending back chunks when requested. We first reject the snapshot
	// with height 2 format 2, and accept the snapshot at height 1.
	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height:      2,
			Format:      2,
			ChunkHashes: [][]byte{{1}},
		},
		AppHash: []byte("app_hash_2"),
	}).Return(&abci.ResponseOfferSnapshot{
		Accepted: false,
		Reason:   abci.ResponseOfferSnapshot_invalid_format,
	}, nil)
	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height:      s.Height,
			Format:      s.Format,
			ChunkHashes: s.ChunkHashes,
			Metadata:    s.Metadata,
		},
		AppHash: []byte("app_hash"),
	}).Return(&abci.ResponseOfferSnapshot{
		Accepted: true,
	}, nil)

	onChunkRequest := func(args mock.Arguments) {
		msg := &chunkRequestMessage{}
		err := cdc.UnmarshalBinaryBare(args[1].([]byte), &msg)
		require.NoError(t, err)
		require.EqualValues(t, 1, msg.Height)
		require.EqualValues(t, 1, msg.Format)
		require.LessOrEqual(t, msg.Index, uint32(len(chunks)))

		added, err := sync.AddChunk(chunks[msg.Index])
		require.NoError(t, err)
		assert.True(t, added)
	}
	peerA.On("Send", ChunkChannel, mock.Anything).Run(onChunkRequest).Return(true)
	peerB.On("Send", ChunkChannel, mock.Anything).Run(onChunkRequest).Return(true)

	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Chunk: []byte{1, 1, 0},
	}).Return(&abci.ResponseApplySnapshotChunk{Applied: true}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Chunk: []byte{1, 1, 1},
	}).Return(&abci.ResponseApplySnapshotChunk{Applied: true}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Chunk: []byte{1, 1, 2},
	}).Return(&abci.ResponseApplySnapshotChunk{Applied: true}, nil)
	connQuery.On("InfoSync", proxy.RequestInfo).Return(&abci.ResponseInfo{
		LastBlockAppHash: []byte("app_hash"),
	}, nil)

	newState, _, err := sync.Sync(state)
	require.NoError(t, err)

	assert.Equal(t, sm.State{
		ChainID:         state.ChainID,
		LastBlockHeight: 1,
		LastBlockTime:   time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		LastBlockID:     types.BlockID{Hash: []byte("blockid")},
		AppHash:         []byte("app_hash"),
		LastResultsHash: []byte("last_results_hash"),
		LastValidators: &types.ValidatorSet{
			Proposer: &types.Validator{Address: []byte("val1")},
		},
		Validators: &types.ValidatorSet{
			Proposer: &types.Validator{Address: []byte("val2")},
		},
		NextValidators: &types.ValidatorSet{
			Proposer: &types.Validator{Address: []byte("val3")},
		},
		LastHeightValidatorsChanged:      1,
		ConsensusParams:                  state.ConsensusParams,
		LastHeightConsensusParamsChanged: state.LastHeightConsensusParamsChanged,
	}, newState)

	connSnapshot.AssertExpectations(t)
	connQuery.AssertExpectations(t)
}

func TestSyncer_AppHashMismatch(t *testing.T) {
	chunks := []*chunk{
		{Height: 1, Format: 1, Index: 0, Body: []byte{1, 1, 0}},
		{Height: 1, Format: 1, Index: 1, Body: []byte{1, 1, 1}},
	}
	s := &snapshot{Height: 1, Format: 1, ChunkHashes: [][]byte{}}
	for _, c := range chunks {
		s.ChunkHashes = append(s.ChunkHashes, c.Hash())
	}

	connSnapshot := &proxymocks.AppConnSnapshot{}
	connQuery := &proxymocks.AppConnQuery{}
	lc := &mockLightClient{}
	lc.On("VerifyHeaderAtHeight", int64(1), mock.Anything).Return(&types.SignedHeader{
		Header: &types.Header{
			Height: 1,
			Time:   time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Commit: &types.Commit{BlockID: types.BlockID{Hash: []byte("blockid")}},
	}, nil)
	lc.On("VerifyHeaderAtHeight", int64(2), mock.Anything).Return(&types.SignedHeader{
		Header: &types.Header{Height: 2, AppHash: []byte("app_hash"), LastResultsHash: []byte("last_results_hash")},
	}, nil)
	lc.On("VerifyHeaderAtHeight", int64(3), mock.Anything).Return(&types.SignedHeader{
		Header: &types.Header{Height: 3},
	}, nil)
	lc.On("TrustedValidatorSet", int64(1)).Return(&types.ValidatorSet{
		Proposer: &types.Validator{Address: []byte("val1")},
	}, int64(1), nil)
	lc.On("TrustedValidatorSet", int64(2)).Return(&types.ValidatorSet{
		Proposer: &types.Validator{Address: []byte("val2")},
	}, int64(2), nil)
	lc.On("TrustedValidatorSet", int64(3)).Return(&types.ValidatorSet{
		Proposer: &types.Validator{Address: []byte("val3")},
	}, int64(3), nil)

	sync := newSyncer(log.NewNopLogger(), connSnapshot, connQuery, lc)

	// A peer reports back with a snapshot
	peer := &p2pmocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	_, err := sync.AddSnapshot(peer, s)
	require.NoError(t, err)

	// We start a sync, with peers sending back chunks when requested
	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height:      s.Height,
			Format:      s.Format,
			ChunkHashes: s.ChunkHashes,
			Metadata:    s.Metadata,
		},
		AppHash: []byte("app_hash"),
	}).Return(&abci.ResponseOfferSnapshot{
		Accepted: true,
	}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Chunk: []byte{1, 1, 0},
	}).Return(&abci.ResponseApplySnapshotChunk{Applied: true}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Chunk: []byte{1, 1, 1},
	}).Return(&abci.ResponseApplySnapshotChunk{Applied: true}, nil)
	connQuery.On("InfoSync", proxy.RequestInfo).Return(&abci.ResponseInfo{
		LastBlockAppHash: []byte("other_app_hash"),
	}, nil)

	onChunkRequest := func(args mock.Arguments) {
		msg := &chunkRequestMessage{}
		err := cdc.UnmarshalBinaryBare(args[1].([]byte), &msg)
		require.NoError(t, err)
		require.EqualValues(t, 1, msg.Height)
		require.EqualValues(t, 1, msg.Format)
		require.LessOrEqual(t, msg.Index, uint32(len(chunks)))

		added, err := sync.AddChunk(chunks[msg.Index])
		require.NoError(t, err)
		assert.True(t, added)
	}
	peer.On("Send", ChunkChannel, mock.Anything).Run(onChunkRequest).Return(true)

	_, _, err = sync.Sync(sm.State{})
	require.Error(t, err)

	connSnapshot.AssertExpectations(t)
	connQuery.AssertExpectations(t)
}

func TestSyncer_NoSnapshots(t *testing.T) {
	connSnapshot := &proxymocks.AppConnSnapshot{}
	connQuery := &proxymocks.AppConnQuery{}
	lc := &mockLightClient{}
	sync := newSyncer(log.NewNopLogger(), connSnapshot, connQuery, lc)

	_, _, err := sync.Sync(sm.State{})
	require.Error(t, err)
	assert.Equal(t, errNoSnapshots, err)
}
