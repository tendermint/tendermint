package statesync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
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

func newTestSyncer(
	t *testing.T,
	shim *p2p.ReactorShim,
	p2pSwitch *p2p.Switch,
	conn proxy.AppConnSnapshot,
	connQuery proxy.AppConnQuery,
	stateProvider StateProvider,
) *syncer {
	t.Helper()

	snapshotCh := shim.GetChannel(p2p.ChannelID(SnapshotChannel))
	chunkCh := shim.GetChannel(p2p.ChannelID(ChunkChannel))
	r := NewReactor(
		shim.Logger,
		p2pSwitch,
		conn,
		connQuery,
		snapshotCh,
		chunkCh,
		shim.PeerUpdateCh,
		"",
	)

	ctx, cancel := context.WithCancel(context.Background())
	go r.Run(ctx)
	require.NoError(t, shim.Start())

	t.Cleanup(func() {
		cancel()

		if err := shim.Stop(); err != nil {
			t.Error(err)
		}
	})

	return newSyncer(log.NewNopLogger(), conn, connQuery, stateProvider, snapshotCh.Out, chunkCh.Out, "")
}

// Sets up a basic syncer that can be used to test OfferSnapshot requests
func setupOfferSyncer(t *testing.T) (*syncer, *proxymocks.AppConnSnapshot) {
	t.Helper()

	connQuery := &proxymocks.AppConnQuery{}
	connSnapshot := &proxymocks.AppConnSnapshot{}

	stateProvider := &mocks.StateProvider{}
	stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

	shim := p2p.NewShim("StateSync", GetChannelShims())

	cfg := config.DefaultP2PConfig()
	p2pSwitch := p2p.MakeSwitch(cfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
		sw.AddReactor(shim.Name, shim)
		return sw
	})

	syncer := newTestSyncer(t, shim, p2pSwitch, connSnapshot, connQuery, stateProvider)

	return syncer, connSnapshot
}

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

	peerA, peerAID := simplePeer(t, "AA")
	peerB, peerBID := simplePeer(t, "BB")

	shim := p2p.NewShim("StateSync", GetChannelShims())

	cfg := config.DefaultP2PConfig()
	p2pSwitch := p2p.MakeSwitch(cfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
		p2p.AddPeerToSwitchPeerSet(sw, peerA)
		p2p.AddPeerToSwitchPeerSet(sw, peerB)
		sw.AddReactor(shim.Name, shim)
		return sw
	})

	syncer := newTestSyncer(t, shim, p2pSwitch, connSnapshot, connQuery, stateProvider)

	// Adding a chunk should error when no sync is in progress
	_, err := syncer.AddChunk(&chunk{Height: 1, Format: 1, Index: 0, Chunk: []byte{1}})
	require.Error(t, err)

	// Adding a couple of peers should trigger snapshot discovery messages
	aReceived := false
	peerA.On("Send", SnapshotChannel, mustEncodeMsg(&ssproto.SnapshotsRequest{})).Run(func(_ mock.Arguments) { aReceived = true }).Return(true)
	syncer.AddPeer(peerAID)
	tryUntil(t, func() bool { return aReceived }, time.Millisecond, time.Second)
	peerA.AssertExpectations(t)

	bReceived := false
	peerB.On("Send", SnapshotChannel, mustEncodeMsg(&ssproto.SnapshotsRequest{})).Run(func(_ mock.Arguments) { bReceived = true }).Return(true)
	syncer.AddPeer(peerBID)
	tryUntil(t, func() bool { return bReceived }, time.Millisecond, time.Second)
	peerB.AssertExpectations(t)

	// Both peers report back with snapshots. One of them also returns a snapshot we don't want, in
	// format 2, which will be rejected by the ABCI application.
	new, err := syncer.AddSnapshot(peerAID, s)
	require.NoError(t, err)
	assert.True(t, new)

	new, err = syncer.AddSnapshot(peerBID, s)
	require.NoError(t, err)
	assert.False(t, new)

	new, err = syncer.AddSnapshot(peerBID, &snapshot{Height: 2, Format: 2, Chunks: 3, Hash: []byte{1}})
	require.NoError(t, err)
	assert.True(t, new)

	// We start a sync, with peers sending back chunks when requested. We first reject the snapshot
	// with height 2 format 2, and accept the snapshot at height 1.
	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height: 2,
			Format: 2,
			Chunks: 3,
			Hash:   []byte{1},
		},
		AppHash: []byte("app_hash_2"),
	}).Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_FORMAT}, nil)
	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
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
	onChunkRequest := func(args mock.Arguments) {
		pb, err := decodeMsg(args[1].([]byte))
		require.NoError(t, err)
		msg := pb.(*ssproto.ChunkRequest)
		require.EqualValues(t, 1, msg.Height)
		require.EqualValues(t, 1, msg.Format)
		require.LessOrEqual(t, msg.Index, uint32(len(chunks)))

		added, err := syncer.AddChunk(chunks[msg.Index])
		require.NoError(t, err)
		assert.True(t, added)

		chunkRequestsMtx.Lock()
		chunkRequests[msg.Index]++
		chunkRequestsMtx.Unlock()
	}

	peerA.On("Send", ChunkChannel, mock.Anything).Maybe().Run(onChunkRequest).Return(true)
	peerB.On("Send", ChunkChannel, mock.Anything).Maybe().Run(onChunkRequest).Return(true)

	// The first time we're applying chunk 2 we tell it to retry the snapshot and discard chunk 1,
	// which should cause it to keep the existing chunk 0 and 2, and restart restoration from
	// beginning. We also wait for a little while, to exercise the retry logic in fetchChunks().
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Index: 2, Chunk: []byte{1, 1, 2},
	}).Once().Run(func(args mock.Arguments) { time.Sleep(2 * time.Second) }).Return(
		&abci.ResponseApplySnapshotChunk{
			Result:        abci.ResponseApplySnapshotChunk_RETRY_SNAPSHOT,
			RefetchChunks: []uint32{1},
		}, nil)

	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Index: 0, Chunk: []byte{1, 1, 0},
	}).Times(2).Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Index: 1, Chunk: []byte{1, 1, 1},
	}).Times(2).Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
	connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
		Index: 2, Chunk: []byte{1, 1, 2},
	}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
	connQuery.On("InfoSync", proxy.RequestInfo).Return(&abci.ResponseInfo{
		AppVersion:       9,
		LastBlockHeight:  1,
		LastBlockAppHash: []byte("app_hash"),
	}, nil)

	newState, lastCommit, err := syncer.SyncAny(0)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // wait for peers to receive requests

	chunkRequestsMtx.Lock()
	assert.Equal(t, map[uint32]int{0: 1, 1: 2, 2: 1}, chunkRequests)
	chunkRequestsMtx.Unlock()

	// The syncer should have updated the state app version from the ABCI info response.
	expectState := state
	expectState.Version.Consensus.App = 9

	assert.Equal(t, expectState, newState)
	assert.Equal(t, commit, lastCommit)

	connSnapshot.AssertExpectations(t)
	connQuery.AssertExpectations(t)
	peerA.AssertExpectations(t)
	peerB.AssertExpectations(t)
}

func TestSyncer_SyncAny_noSnapshots(t *testing.T) {
	syncer, _ := setupOfferSyncer(t)
	_, _, err := syncer.SyncAny(0)
	assert.Equal(t, errNoSnapshots, err)
}

func TestSyncer_SyncAny_abort(t *testing.T) {
	syncer, connSnapshot := setupOfferSyncer(t)

	s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	p, pID := simplePeer(t, "FF")

	pID, err := p2p.PeerIDFromString(string(p.ID()))
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s)
	require.NoError(t, err)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}, nil)

	_, _, err = syncer.SyncAny(0)
	assert.Equal(t, errAbort, err)
	connSnapshot.AssertExpectations(t)
}

func TestSyncer_SyncAny_reject(t *testing.T) {
	syncer, connSnapshot := setupOfferSyncer(t)

	// s22 is tried first, then s12, then s11, then errNoSnapshots
	s22 := &snapshot{Height: 2, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s12 := &snapshot{Height: 1, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s11 := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	p, pID := simplePeer(t, "FF")

	pID, err := p2p.PeerIDFromString(string(p.ID()))
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s22)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s12)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s11)
	require.NoError(t, err)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s22), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s12), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s11), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	_, _, err = syncer.SyncAny(0)
	assert.Equal(t, errNoSnapshots, err)
	connSnapshot.AssertExpectations(t)
}

func TestSyncer_SyncAny_reject_format(t *testing.T) {
	syncer, connSnapshot := setupOfferSyncer(t)

	// s22 is tried first, which reject s22 and s12, then s11 will abort.
	s22 := &snapshot{Height: 2, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s12 := &snapshot{Height: 1, Format: 2, Chunks: 3, Hash: []byte{1, 2, 3}}
	s11 := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	p, pID := simplePeer(t, "FF")

	pID, err := p2p.PeerIDFromString(string(p.ID()))
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s22)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s12)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s11)
	require.NoError(t, err)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s22), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_FORMAT}, nil)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s11), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}, nil)

	_, _, err = syncer.SyncAny(0)
	assert.Equal(t, errAbort, err)
	connSnapshot.AssertExpectations(t)
}

func TestSyncer_SyncAny_reject_sender(t *testing.T) {
	syncer, connSnapshot := setupOfferSyncer(t)

	_, peerAID := simplePeer(t, "AA")
	_, peerBID := simplePeer(t, "BB")
	_, peerCID := simplePeer(t, "CC")

	// sbc will be offered first, which will be rejected with reject_sender, causing all snapshots
	// submitted by both b and c (i.e. sb, sc, sbc) to be rejected. Finally, sa will reject and
	// errNoSnapshots is returned.
	sa := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	sb := &snapshot{Height: 2, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	sc := &snapshot{Height: 3, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}
	sbc := &snapshot{Height: 4, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	_, err := syncer.AddSnapshot(peerAID, sa)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(peerBID, sb)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(peerCID, sc)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(peerBID, sbc)
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(peerCID, sbc)
	require.NoError(t, err)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(sbc), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_SENDER}, nil)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(sa), AppHash: []byte("app_hash"),
	}).Once().Return(&abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil)

	_, _, err = syncer.SyncAny(0)
	assert.Equal(t, errNoSnapshots, err)
	connSnapshot.AssertExpectations(t)
}

func TestSyncer_SyncAny_abciError(t *testing.T) {
	syncer, connSnapshot := setupOfferSyncer(t)

	errBoom := errors.New("boom")
	s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}}

	p, pID := simplePeer(t, "FF")

	pID, err := p2p.PeerIDFromString(string(p.ID()))
	require.NoError(t, err)

	_, err = syncer.AddSnapshot(pID, s)
	require.NoError(t, err)

	connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
		Snapshot: toABCI(s), AppHash: []byte("app_hash"),
	}).Once().Return(nil, errBoom)

	_, _, err = syncer.SyncAny(0)
	assert.True(t, errors.Is(err, errBoom))
	connSnapshot.AssertExpectations(t)
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
			syncer, connSnapshot := setupOfferSyncer(t)
			s := &snapshot{Height: 1, Format: 1, Chunks: 3, Hash: []byte{1, 2, 3}, trustedAppHash: []byte("app_hash")}
			connSnapshot.On("OfferSnapshotSync", abci.RequestOfferSnapshot{
				Snapshot: toABCI(s),
				AppHash:  []byte("app_hash"),
			}).Return(&abci.ResponseOfferSnapshot{Result: tc.result}, tc.err)
			err := syncer.offerSnapshot(s)
			if tc.expectErr == unknownErr {
				require.Error(t, err)
			} else {
				unwrapped := errors.Unwrap(err)
				if unwrapped != nil {
					err = unwrapped
				}
				assert.Equal(t, tc.expectErr, err)
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
			connQuery := &proxymocks.AppConnQuery{}
			connSnapshot := &proxymocks.AppConnSnapshot{}
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			shim := p2p.NewShim("StateSync", GetChannelShims())

			cfg := config.DefaultP2PConfig()
			p2pSwitch := p2p.MakeSwitch(cfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
				sw.AddReactor(shim.Name, shim)
				return sw
			})

			syncer := newTestSyncer(t, shim, p2pSwitch, connSnapshot, connQuery, stateProvider)

			body := []byte{1, 2, 3}
			chunks, err := newChunkQueue(&snapshot{Height: 1, Format: 1, Chunks: 1}, "")
			require.NoError(t, err)
			_, err = chunks.Add(&chunk{Height: 1, Format: 1, Index: 0, Chunk: body})
			require.NoError(t, err)

			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 0, Chunk: body,
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: tc.result}, tc.err)
			if tc.result == abci.ResponseApplySnapshotChunk_RETRY {
				connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
					Index: 0, Chunk: body,
				}).Once().Return(&abci.ResponseApplySnapshotChunk{
					Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			}

			err = syncer.applyChunks(chunks)
			if tc.expectErr == unknownErr {
				require.Error(t, err)
			} else {
				unwrapped := errors.Unwrap(err)
				if unwrapped != nil {
					err = unwrapped
				}
				assert.Equal(t, tc.expectErr, err)
			}
			connSnapshot.AssertExpectations(t)
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
			connQuery := &proxymocks.AppConnQuery{}
			connSnapshot := &proxymocks.AppConnSnapshot{}
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			shim := p2p.NewShim("StateSync", GetChannelShims())

			cfg := config.DefaultP2PConfig()
			p2pSwitch := p2p.MakeSwitch(cfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
				sw.AddReactor(shim.Name, shim)
				return sw
			})

			syncer := newTestSyncer(t, shim, p2pSwitch, connSnapshot, connQuery, stateProvider)

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
			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 0, Chunk: []byte{0},
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 1, Chunk: []byte{1},
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 2, Chunk: []byte{2},
			}).Once().Return(&abci.ResponseApplySnapshotChunk{
				Result:        tc.result,
				RefetchChunks: []uint32{1},
			}, nil)

			// Since removing the chunk will cause Next() to block, we spawn a goroutine, then
			// check the queue contents, and finally close the queue to end the goroutine.
			// We don't really care about the result of applyChunks, since it has separate test.
			go func() {
				syncer.applyChunks(chunks) //nolint:errcheck // purposefully ignore error
			}()

			time.Sleep(50 * time.Millisecond)
			assert.True(t, chunks.Has(0))
			assert.False(t, chunks.Has(1))
			assert.True(t, chunks.Has(2))
			err = chunks.Close()
			require.NoError(t, err)
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
			connQuery := &proxymocks.AppConnQuery{}
			connSnapshot := &proxymocks.AppConnSnapshot{}
			stateProvider := &mocks.StateProvider{}
			stateProvider.On("AppHash", mock.Anything, mock.Anything).Return([]byte("app_hash"), nil)

			shim := p2p.NewShim("StateSync", GetChannelShims())

			cfg := config.DefaultP2PConfig()
			p2pSwitch := p2p.MakeSwitch(cfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
				sw.AddReactor(shim.Name, shim)
				return sw
			})

			syncer := newTestSyncer(t, shim, p2pSwitch, connSnapshot, connQuery, stateProvider)

			// Set up three peers across two snapshots, and ask for one of them to be banned.
			// It should be banned from all snapshots.
			_, peerAID := simplePeer(t, "AA")
			_, peerBID := simplePeer(t, "BB")
			_, peerCID := simplePeer(t, "CC")

			s1 := &snapshot{Height: 1, Format: 1, Chunks: 3}
			s2 := &snapshot{Height: 2, Format: 1, Chunks: 3}

			_, err := syncer.AddSnapshot(peerAID, s1)
			require.NoError(t, err)

			_, err = syncer.AddSnapshot(peerAID, s2)
			require.NoError(t, err)

			_, err = syncer.AddSnapshot(peerBID, s1)
			require.NoError(t, err)

			_, err = syncer.AddSnapshot(peerBID, s2)
			require.NoError(t, err)

			_, err = syncer.AddSnapshot(peerCID, s1)
			require.NoError(t, err)

			_, err = syncer.AddSnapshot(peerCID, s2)
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
			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 0, Chunk: []byte{0}, Sender: "AA",
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 1, Chunk: []byte{1}, Sender: "BB",
			}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
				Index: 2, Chunk: []byte{2}, Sender: "CC",
			}).Once().Return(&abci.ResponseApplySnapshotChunk{
				Result:        tc.result,
				RejectSenders: []string{peerBID.String()},
			}, nil)

			// On retry, the last chunk will be tried again, so we just accept it then.
			if tc.result == abci.ResponseApplySnapshotChunk_RETRY {
				connSnapshot.On("ApplySnapshotChunkSync", abci.RequestApplySnapshotChunk{
					Index: 2, Chunk: []byte{2}, Sender: "CC",
				}).Once().Return(&abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil)
			}

			// We don't really care about the result of applyChunks, since it has separate test.
			// However, it will block on e.g. retry result, so we spawn a goroutine that will
			// be shut down when the chunk queue closes.
			go func() {
				syncer.applyChunks(chunks) //nolint:errcheck // purposefully ignore error
			}()

			time.Sleep(50 * time.Millisecond)

			s1peers := syncer.snapshots.GetPeers(s1)
			require.Len(t, s1peers, 2)
			require.EqualValues(t, "AA", s1peers[0].String())
			require.EqualValues(t, "CC", s1peers[1].String())

			syncer.snapshots.GetPeers(s1)
			require.Len(t, s1peers, 2)
			require.EqualValues(t, "AA", s1peers[0].String())
			require.EqualValues(t, "CC", s1peers[1].String())

			err = chunks.Close()
			require.NoError(t, err)
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
			connQuery := &proxymocks.AppConnQuery{}
			connSnapshot := &proxymocks.AppConnSnapshot{}
			stateProvider := &mocks.StateProvider{}

			shim := p2p.NewShim("StateSync", GetChannelShims())

			cfg := config.DefaultP2PConfig()
			p2pSwitch := p2p.MakeSwitch(cfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
				sw.AddReactor(shim.Name, shim)
				return sw
			})

			syncer := newTestSyncer(t, shim, p2pSwitch, connSnapshot, connQuery, stateProvider)

			connQuery.On("InfoSync", proxy.RequestInfo).Return(tc.response, tc.err)
			version, err := syncer.verifyApp(s)
			unwrapped := errors.Unwrap(err)
			if unwrapped != nil {
				err = unwrapped
			}
			assert.Equal(t, tc.expectErr, err)
			if err == nil {
				assert.Equal(t, tc.response.AppVersion, version)
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
