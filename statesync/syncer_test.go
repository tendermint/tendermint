package statesync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/proxy"
	proxymocks "github.com/tendermint/tendermint/proxy/mocks"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/statesync/mocks"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

var ctx = context.Background()

func TestSyncer_SyncAny(t *testing.T) {
	state := sm.State{
		ChainID: "chain",
		Version: tmstate.Version{
			Consensus: tmversion.Consensus{
				Block: version.BlockProtocol,
				App:   0,
			},
			Software: version.TMCoreSemVer,
		},

		LastBlockHeight: 1,
		LastBlockID:     types.BlockID{Hash: []byte("blockhash")},
		LastBlockTime:   time.Now(),
		LastResultsHash: []byte("last_results_hash"),
		AppHash:         []byte("app_hash"),

		LastValidators: &types.ValidatorSet{Proposer: &types.Validator{Address: []byte("val1")}},
		Validators:     &types.ValidatorSet{Proposer: &types.Validator{Address: []byte("val2")}},
		NextValidators: &types.ValidatorSet{Proposer: &types.Validator{Address: []byte("val3")}},

		ConsensusParams:                  *types.DefaultConsensusParams(),
		LastHeightConsensusParamsChanged: 1,
	}
	commit := &types.Commit{BlockID: types.BlockID{Hash: []byte("blockhash")}}

	chunks := []*chunk{
		{Height: 1, Format: 1, Index: 0, Chunk: []byte{1, 1, 0}},
		{Height: 1, Format: 1, Index: 1, Chunk: []byte{1, 1, 1}},
		{Height: 1, Format: 1, Index: 2, Chunk: []byte{1, 1, 2}},
	}
	s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, uint64(1)).Return(state.AppHash, nil)
	stateProvider.On("AppHash", mock.Anything, uint64(2)).Return([]byte("app_hash_2"), nil)
	stateProvider.On("Commit", mock.Anything, uint64(1)).Return(commit, nil)
	stateProvider.On("State", mock.Anything, uint64(1)).Return(state, nil)
	connSnapshot := &proxymocks.AppConnSnapshot{}
	connQuery := &proxymocks.AppConnQuery{}

	peerAID := p2p.NodeID("aa")
	peerBID := p2p.NodeID("bb")

	rts := setup(t, connSnapshot, connQuery, stateProvider, 3)

	// Adding a chunk should error when no sync is in progress
	_, err := rts.syncer.AddChunk(&chunk{Height: 1, Format: 1, Index: 0, Chunk: []byte{1}})
	require.Error(t, err)

	// Adding a couple of peers should trigger snapshot discovery messages
	rts.syncer.AddPeer(peerAID)
	e := <-rts.snapshotOutCh
	require.Equal(t, &ssproto.SnapshotsRequest{}, e.Message)
	require.Equal(t, peerAID, e.To)

	rts.syncer.AddPeer(peerBID)
	e = <-rts.snapshotOutCh
	require.Equal(t, &ssproto.SnapshotsRequest{}, e.Message)
	require.Equal(t, peerBID, e.To)

	// Both peers report back with snapshots. One of them also returns a snapshot we don't want, in
	// format 2, which will be rejected by the ABCI application.
	new, err := rts.syncer.AddSnapshot(peerAID, s)
	require.NoError(t, err)
	require.True(t, new)

	new, err = rts.syncer.AddSnapshot(peerBID, s)
	require.NoError(t, err)
	require.False(t, new)

	new, err = rts.syncer.AddSnapshot(peerBID, &snapshot{Height: 2, Format: 2, Chunks: 3, Hash: []byte{1}})
	require.NoError(t, err)
	require.True(t, new)

	// We start a sync, with peers sending back chunks when requested. We first reject the snapshot
	// with height 2 format 2, and accept the snapshot at height 1.
	connSnapshot.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height: 2,
			Format: 2,
			Chunks: 3,
			Hash:   []byte{1},
		},
		AppHash: []byte("app_hash_2"),
	}).Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_FORMAT}, nil)
	connSnapshot.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height:   s.Height,
			Format:   s.Format,
			Chunks:   s.Chunks,
			Hash:     s.Hash,
			Metadata: s.Metadata,
		},
		AppHash: []byte("app_hash"),
	}).Times(2).Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}, nil)

	chunkRequests := make(map[uint32]int)
	chunkRequestsMtx := tmsync.Mutex{}

	go func() {
		for e := range rts.chunkOutCh {
			msg, ok := e.Message.(*ssproto.ChunkRequest)
			require.True(t, ok)

			require.EqualValues(t, 1, msg.Height)
			require.EqualValues(t, 1, msg.Format)
			require.LessOrEqual(t, msg.Index, uint32(len(chunks)))

			added, err := rts.syncer.AddChunk(chunks[msg.Index])
			require.NoError(t, err)
			require.True(t, added)

			chunkRequestsMtx.Lock()
			chunkRequests[msg.Index]++
			chunkRequestsMtx.Unlock()
		}
	}()

	// The first time we're applying chunk 2 we tell it to retry the snapshot and discard chunk 1,
	// which should cause it to keep the existing chunk 0 and 2, and restart restoration from
	// beginning. We also wait for a little while, to exercise the retry logic in fetchChunks().
	connSnapshot.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
		Index: 2, Chunk: []byte{1, 1, 2},
	}).Once().Run(func(args mock.Arguments) { time.Sleep(2 * time.Second) }).Return(
		&abci.ResponseApplySnapshotChunk{
			Result:        abci.ResponseApplySnapshotChunk_RETRY_SNAPSHOT,
			RefetchChunks: []uint32{1},
		}, nil)

	connSnapshot.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
		Index: 0, Chunk: []byte{1, 1, 0},
	}).Times(2).Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
		Index: 1, Chunk: []byte{1, 1, 1},
	}).Times(2).Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
		Index: 2, Chunk: []byte{1, 1, 2},
	}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
	connQuery.On("InfoSync", ctx, proxy.RequestInfo).Return(&abci.ResponseInfo{
		AppVersion:       9,
		LastBlockHeight:  1,
		LastBlockAppHash: []byte("app_hash"),
	}, nil)

	newState, lastCommit, err := rts.syncer.SyncAny(0)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // wait for peers to receive requests

	chunkRequestsMtx.Lock()
	require.Equal(t, map[uint32]int{0: 1, 1: 2, 2: 1}, chunkRequests)
	chunkRequestsMtx.Unlock()

	// The syncer should have updated the state app version from the ABCI info response.
	expectState := state
	expectState.Version.Consensus.App = 9

	require.Equal(t, expectState, newState)
	require.Equal(t, commit, lastCommit)

	connSnapshot.AssertExpectations(t)
	connQuery.AssertExpectations(t)
}

func TestSyncer_SyncAny_noSnapshots(t *testing.T) {
	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	rts := setup(t, nil, nil, stateProvider, 2)

	_, _, err := rts.syncer.SyncAny(0)
	require.Equal(t, errNoSnapshots, err)
}

func TestSyncer_SyncAny_abort(t *testing.T) {
	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	rts := setup(t, nil, nil, stateProvider, 2)

	s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	peerID := p2p.NodeID("aa")

	_, err := rts.syncer.AddSnapshot(peerID, s)
	require.NoError(t, err)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}, nil)

	_, _, err = rts.syncer.SyncAny(0)
	require.Equal(t, errAbort, err)
	rts.conn.AssertExpectations(t)
}

func TestSyncer_SyncAny_reject(t *testing.T) {
	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	rts := setup(t, nil, nil, stateProvider, 2)

	// s22 is tried first, then s12, then s11, then errNoSnapshots
	s22 := &snapshot{Height: 2, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s12 := &snapshot{Height: 1, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s11 := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	peerID := p2p.NodeID("aa")

	_, err := rts.syncer.AddSnapshot(peerID, s22)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerID, s12)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerID, s11)
	require.NoError(t, err)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s22), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s12), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s11), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	_, _, err = rts.syncer.SyncAny(0)
	require.Equal(t, errNoSnapshots, err)
	rts.conn.AssertExpectations(t)
}

func TestSyncer_SyncAny_reject_format(t *testing.T) {
	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	rts := setup(t, nil, nil, stateProvider, 2)

	// s22 is tried first, which reject s22 and s12, then s11 will abort.
	s22 := &snapshot{Height: 2, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s12 := &snapshot{Height: 1, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s11 := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	peerID := p2p.NodeID("aa")

	_, err := rts.syncer.AddSnapshot(peerID, s22)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerID, s12)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerID, s11)
	require.NoError(t, err)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s22), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_FORMAT}, nil)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s11), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}, nil)

	_, _, err = rts.syncer.SyncAny(0)
	require.Equal(t, errAbort, err)
	rts.conn.AssertExpectations(t)
}

func TestSyncer_SyncAny_reject_sender(t *testing.T) {
	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	rts := setup(t, nil, nil, stateProvider, 2)

	peerAID := p2p.NodeID("aa")
	peerBID := p2p.NodeID("bb")
	peerCID := p2p.NodeID("cc")

	// sbc will be offered first, which will be rejected with reject_sender, causing all snapshots
	// submitted by both b and c (i.e. sb, sc, sbc) to be rejected. Finally, sa will reject and
	// errNoSnapshots is returned.
	sa := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	sb := &snapshot{Height: 2, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	sc := &snapshot{Height: 3, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	sbc := &snapshot{Height: 4, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	_, err := rts.syncer.AddSnapshot(peerAID, sa)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerBID, sb)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerCID, sc)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerBID, sbc)
	require.NoError(t, err)

	_, err = rts.syncer.AddSnapshot(peerCID, sbc)
	require.NoError(t, err)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(sbc), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_SENDER}, nil)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(sa), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	_, _, err = rts.syncer.SyncAny(0)
	require.Equal(t, errNoSnapshots, err)
	rts.conn.AssertExpectations(t)
}

func TestSyncer_SyncAny_abciError(t *testing.T) {
	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	rts := setup(t, nil, nil, stateProvider, 2)

	errBoom := errors.New("boom")
	s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	peerID := p2p.NodeID("aa")

	_, err := rts.syncer.AddSnapshot(peerID, s)
	require.NoError(t, err)

	rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
		Snapshot: toABCI(s), AppHash: []byte("app_hash"),
	}).Once().Return(nil, errBoom)

	_, _, err = rts.syncer.SyncAny(0)
	require.True(t, errors.Is(err, errBoom))
	rts.conn.AssertExpectations(t)
}

func TestSyncer_offerSnapshot(t *testing.T) {
	unknownErr := errors.New("unknown error")
	boom := errors.New("boom")

	testcases := map[string]struct {
		result    abci.ResponseOfferSnapshot_Result
		err       error
		expectErr error
	}{
		"accept":           {abci.ResponseOfferSnapshot_ACCEPT, nil, nil},
		"abort":            {abci.ResponseOfferSnapshot_ABORT, nil, errAbort},
		"reject":           {abci.ResponseOfferSnapshot_REJECT, nil, errRejectSnapshot},
		"reject_format":    {abci.ResponseOfferSnapshot_REJECT_FORMAT, nil, errRejectFormat},
		"reject_sender":    {abci.ResponseOfferSnapshot_REJECT_SENDER, nil, errRejectSender},
		"unknown":          {abci.ResponseOfferSnapshot_UNKNOWN, nil, unknownErr},
		"error":            {0, boom, boom},
		"unknown non-zero": {9, nil, unknownErr},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			rts := setup(t, nil, nil, stateProvider, 2)

			s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}, trustedAppHash: []byte("app_hash")}
			rts.conn.On("OfferSnapshotSync", ctx, abci.RequestOfferSnapshot{
				Snapshot: toABCI(s),
				AppHash:  []byte("app_hash"),
			}).Return(&abci.ResponseOfferSnapshot{Result: tc.result}, tc.err)

			err := rts.syncer.offerSnapshot(s)
			if tc.expectErr == unknownErr {
				require.Error(t, err)
			} else {
				unwrapped := errors.Unwrap(err)
				if unwrapped != nil {
					err = unwrapped
				}
				require.Equal(t, tc.expectErr, err)
			}
		})
	}
}

func TestSyncer_applyChunks_Results(t *testing.T) {
	unknownErr := errors.New("unknown error")
	boom := errors.New("boom")

	testcases := map[string]struct {
		result    abci.ResponseApplySnapshotChunk_Result
		err       error
		expectErr error
	}{
		"accept":           {abci.ResponseApplySnapshotChunk_ACCEPT, nil, nil},
		"abort":            {abci.ResponseApplySnapshotChunk_ABORT, nil, errAbort},
		"retry":            {abci.ResponseApplySnapshotChunk_RETRY, nil, nil},
		"retry_snapshot":   {abci.ResponseApplySnapshotChunk_RETRY_SNAPSHOT, nil, errRetrySnapshot},
		"reject_snapshot":  {abci.ResponseApplySnapshotChunk_REJECT_SNAPSHOT, nil, errRejectSnapshot},
		"unknown":          {abci.ResponseApplySnapshotChunk_UNKNOWN, nil, unknownErr},
		"error":            {0, boom, boom},
		"unknown non-zero": {9, nil, unknownErr},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			rts := setup(t, nil, nil, stateProvider, 2)

			body := []byte{1, 2, 3}
			chunks, err := newChunkQueue(&snapshot{Height: 1, Format: 1, Chunks: 1}, "")
			require.NoError(t, err)
			_, err = chunks.Add(&chunk{Height: 1, Format: 1, Index: 0, Chunk: body})
			require.NoError(t, err)

			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 0, Chunk: body,
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: tc.result}, tc.err)
			if tc.result == abci.ResponseApplySnapshotChunk_RETRY {
				rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
					Index: 0, Chunk: body,
				}).Once().Return(&abci.ResponseApplySnapshotChunk{
					Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			}

			err = rts.syncer.applyChunks(chunks)
			if tc.expectErr == unknownErr {
				require.Error(t, err)
			} else {
				unwrapped := errors.Unwrap(err)
				if unwrapped != nil {
					err = unwrapped
				}
				require.Equal(t, tc.expectErr, err)
			}

			rts.conn.AssertExpectations(t)
		})
	}
}

func TestSyncer_applyChunks_RefetchChunks(t *testing.T) {
	// Discarding chunks via refetch_chunks should work the same for all results
	testcases := map[string]struct {
		result abci.ResponseApplySnapshotChunk_Result
	}{
		"accept":          {abci.ResponseApplySnapshotChunk_ACCEPT},
		"abort":           {abci.ResponseApplySnapshotChunk_ABORT},
		"retry":           {abci.ResponseApplySnapshotChunk_RETRY},
		"retry_snapshot":  {abci.ResponseApplySnapshotChunk_RETRY_SNAPSHOT},
		"reject_snapshot": {abci.ResponseApplySnapshotChunk_REJECT_SNAPSHOT},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			rts := setup(t, nil, nil, stateProvider, 2)

			chunks, err := newChunkQueue(&snapshot{Height: 1, Format: 1, Chunks: 3}, "")
			require.NoError(t, err)
			added, err := chunks.Add(&chunk{Height: 1, Format: 1, Index: 0, Chunk: []byte{0}})
			require.True(t, added)
			require.NoError(t, err)
			added, err = chunks.Add(&chunk{Height: 1, Format: 1, Index: 1, Chunk: []byte{1}})
			require.True(t, added)
			require.NoError(t, err)
			added, err = chunks.Add(&chunk{Height: 1, Format: 1, Index: 2, Chunk: []byte{2}})
			require.True(t, added)
			require.NoError(t, err)

			// The first two chunks are accepted, before the last one asks for 1 to be refetched
			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 0, Chunk: []byte{0},
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 1, Chunk: []byte{1},
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 2, Chunk: []byte{2},
			}).Once().Return(&abci.ResponseApplySnapshotChunk{
				Result:        tc.result,
				RefetchChunks: []uint32{1},
			}, nil)

			// Since removing the chunk will cause Next() to block, we spawn a goroutine, then
			// check the queue contents, and finally close the queue to end the goroutine.
			// We don't really care about the result of applyChunks, since it has separate test.
			go func() {
				rts.syncer.applyChunks(chunks) //nolint:errcheck // purposefully ignore error
			}()

			time.Sleep(50 * time.Millisecond)
			require.True(t, chunks.Has(0))
			require.False(t, chunks.Has(1))
			require.True(t, chunks.Has(2))

			require.NoError(t, chunks.Close())
		})
	}
}

func TestSyncer_applyChunks_RejectSenders(t *testing.T) {
	// Banning chunks senders via ban_chunk_senders should work the same for all results
	testcases := map[string]struct {
		result abci.ResponseApplySnapshotChunk_Result
	}{
		"accept":          {abci.ResponseApplySnapshotChunk_ACCEPT},
		"abort":           {abci.ResponseApplySnapshotChunk_ABORT},
		"retry":           {abci.ResponseApplySnapshotChunk_RETRY},
		"retry_snapshot":  {abci.ResponseApplySnapshotChunk_RETRY_SNAPSHOT},
		"reject_snapshot": {abci.ResponseApplySnapshotChunk_REJECT_SNAPSHOT},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			rts := setup(t, nil, nil, stateProvider, 2)

			// Set up three peers across two snapshots, and ask for one of them to be banned.
			// It should be banned from all snapshots.
			peerAID := p2p.NodeID("aa")
			peerBID := p2p.NodeID("bb")
			peerCID := p2p.NodeID("cc")

			s1 := &snapshot{Height: 1, Format: 1, Chunks: 3}
			s2 := &snapshot{Height: 2, Format: 1, Chunks: 3}

			_, err := rts.syncer.AddSnapshot(peerAID, s1)
			require.NoError(t, err)

			_, err = rts.syncer.AddSnapshot(peerAID, s2)
			require.NoError(t, err)

			_, err = rts.syncer.AddSnapshot(peerBID, s1)
			require.NoError(t, err)

			_, err = rts.syncer.AddSnapshot(peerBID, s2)
			require.NoError(t, err)

			_, err = rts.syncer.AddSnapshot(peerCID, s1)
			require.NoError(t, err)

			_, err = rts.syncer.AddSnapshot(peerCID, s2)
			require.NoError(t, err)

			chunks, err := newChunkQueue(s1, "")
			require.NoError(t, err)

			added, err := chunks.Add(&chunk{Height: 1, Format: 1, Index: 0, Chunk: []byte{0}, Sender: peerAID})
			require.True(t, added)
			require.NoError(t, err)

			added, err = chunks.Add(&chunk{Height: 1, Format: 1, Index: 1, Chunk: []byte{1}, Sender: peerBID})
			require.True(t, added)
			require.NoError(t, err)

			added, err = chunks.Add(&chunk{Height: 1, Format: 1, Index: 2, Chunk: []byte{2}, Sender: peerCID})
			require.True(t, added)
			require.NoError(t, err)

			// The first two chunks are accepted, before the last one asks for b sender to be rejected
			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 0, Chunk: []byte{0}, Sender: "aa",
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 1, Chunk: []byte{1}, Sender: "bb",
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
				Index: 2, Chunk: []byte{2}, Sender: "cc",
			}).Once().Return(&abci.ResponseApplySnapshotChunk{
				Result:        tc.result,
				RejectSenders: []string{string(peerBID)},
			}, nil)

			// On retry, the last chunk will be tried again, so we just accept it then.
			if tc.result == abci.ResponseApplySnapshotChunk_RETRY {
				rts.conn.On("ApplySnapshotChunkSync", ctx, abci.RequestApplySnapshotChunk{
					Index: 2, Chunk: []byte{2}, Sender: "cc",
				}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			}

			// We don't really care about the result of applyChunks, since it has separate test.
			// However, it will block on e.g. retry result, so we spawn a goroutine that will
			// be shut down when the chunk queue closes.
			go func() {
				rts.syncer.applyChunks(chunks) //nolint:errcheck // purposefully ignore error
			}()

			time.Sleep(50 * time.Millisecond)

			s1peers := rts.syncer.snapshots.GetPeers(s1)
			require.Len(t, s1peers, 2)
			require.EqualValues(t, "aa", s1peers[0])
			require.EqualValues(t, "cc", s1peers[1])

			rts.syncer.snapshots.GetPeers(s1)
			require.Len(t, s1peers, 2)
			require.EqualValues(t, "aa", s1peers[0])
			require.EqualValues(t, "cc", s1peers[1])

			require.NoError(t, chunks.Close())
		})
	}
}

func TestSyncer_verifyApp(t *testing.T) {
	boom := errors.New("boom")
	s := &snapshot{Height: 3, Format: 1, Chunks: 5, Hash: []byte{1, 2, 3}, trustedAppHash: []byte("app_hash")}

	testcases := map[string]struct {
		response  *abci.ResponseInfo
		err       error
		expectErr error
	}{
		"verified": {&abci.ResponseInfo{
			LastBlockHeight:  3,
			LastBlockAppHash: []byte("app_hash"),
			AppVersion:       9,
		}, nil, nil},
		"invalid height": {&abci.ResponseInfo{
			LastBlockHeight:  5,
			LastBlockAppHash: []byte("app_hash"),
			AppVersion:       9,
		}, nil, errVerifyFailed},
		"invalid hash": {&abci.ResponseInfo{
			LastBlockHeight:  3,
			LastBlockAppHash: []byte("xxx"),
			AppVersion:       9,
		}, nil, errVerifyFailed},
		"error": {nil, boom, boom},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			rts := setup(t, nil, nil, nil, 2)

			rts.connQuery.On("InfoSync", ctx, proxy.RequestInfo).Return(tc.response, tc.err)
			version, err := rts.syncer.verifyApp(s)
			unwrapped := errors.Unwrap(err)
			if unwrapped != nil {
				err = unwrapped
			}

			require.Equal(t, tc.expectErr, err)
			if err == nil {
				require.Equal(t, tc.response.AppVersion, version)
			}
		})
	}
}

func toABCI(s *snapshot) *abci.Snapshot {
	return &abci.Snapshot{
		Height:   s.Height,
		Format:   s.Format,
		Chunks:   s.Chunks,
		Hash:     s.Hash,
		Metadata: s.Metadata,
	}
}
