package statesync

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"
)

func TestSnapshot_Key(t *testing.T) {
	testcases := map[string]struct {
		modify func(*snapshot)
	}{
		"new height":      {func(s *snapshot) { s.Height = 9 }},
		"new format":      {func(s *snapshot) { s.Format = 9 }},
		"new chunk count": {func(s *snapshot) { s.Chunks = 9 }},
		"new hash":        {func(s *snapshot) { s.Hash = []byte{9} }},
		"no metadata":     {func(s *snapshot) { s.Metadata = nil }},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			s := snapshot{
				Height:   3,
				Format:   1,
				Chunks:   7,
				Hash:     []byte{1, 2, 3},
				Metadata: []byte{255},
			}
			before := s.Key()
			tc.modify(&s)
			after := s.Key()
			require.NotEqual(t, before, after)
		})
	}
}

func TestSnapshotPool_Add(t *testing.T) {
	peerID := types.NodeID("aa")

	// Adding to the pool should work
	pool := newSnapshotPool()
	added, err := pool.Add(peerID, &snapshot{
		Height: 1,
		Format: 1,
		Chunks: 1,
		Hash:   []byte{1},
	})
	require.NoError(t, err)
	require.True(t, added)

	// Adding again from a different peer should return false
	otherNodeID := types.NodeID("bb")
	added, err = pool.Add(otherNodeID, &snapshot{
		Height: 1,
		Format: 1,
		Chunks: 1,
		Hash:   []byte{1},
	})
	require.NoError(t, err)
	require.False(t, added)

	snapshot := pool.Best()
	require.NotNil(t, snapshot)
}

func TestSnapshotPool_GetPeer(t *testing.T) {
	pool := newSnapshotPool()

	s := &snapshot{Height: 1, Format: 1, Chunks: 1, Hash: []byte{1}}

	peerAID := types.NodeID("aa")
	peerBID := types.NodeID("bb")

	_, err := pool.Add(peerAID, s)
	require.NoError(t, err)

	_, err = pool.Add(peerBID, s)
	require.NoError(t, err)

	_, err = pool.Add(peerAID, &snapshot{Height: 2, Format: 1, Chunks: 1, Hash: []byte{1}})
	require.NoError(t, err)

	// GetPeer currently picks a random peer, so lets run it until we've seen both.
	seenA := false
	seenB := false
	for !seenA || !seenB {
		peer := pool.GetPeer(s)
		if peer == peerAID {
			seenA = true
		}
		if peer == peerBID {
			seenB = true
		}
	}

	// GetPeer should return empty for an unknown snapshot
	peer := pool.GetPeer(&snapshot{Height: 9, Format: 9})
	require.EqualValues(t, "", peer)
}

func TestSnapshotPool_GetPeers(t *testing.T) {
	pool := newSnapshotPool()

	s := &snapshot{Height: 1, Format: 1, Chunks: 1, Hash: []byte{1}}

	peerAID := types.NodeID("aa")
	peerBID := types.NodeID("bb")

	_, err := pool.Add(peerAID, s)
	require.NoError(t, err)

	_, err = pool.Add(peerBID, s)
	require.NoError(t, err)

	_, err = pool.Add(peerAID, &snapshot{Height: 2, Format: 1, Chunks: 1, Hash: []byte{2}})
	require.NoError(t, err)

	peers := pool.GetPeers(s)
	require.Len(t, peers, 2)
	require.Equal(t, peerAID, peers[0])
	require.EqualValues(t, peerBID, peers[1])
}

func TestSnapshotPool_Ranked_Best(t *testing.T) {
	pool := newSnapshotPool()

	// snapshots in expected order (best to worst). Highest height wins, then highest format.
	// Snapshots with different chunk hashes are considered different, and the most peers is
	// tie-breaker.
	expectSnapshots := []struct {
		snapshot *snapshot
		peers    []types.NodeID
	}{
		{&snapshot{Height: 2, Format: 2, Chunks: 4, Hash: []byte{1, 3}}, []types.NodeID{"AA", "BB", "CC", "DD"}},
		{&snapshot{Height: 1, Format: 1, Chunks: 4, Hash: []byte{1, 2}}, []types.NodeID{"AA", "BB", "CC", "DD"}},
		{&snapshot{Height: 2, Format: 2, Chunks: 5, Hash: []byte{1, 2}}, []types.NodeID{"AA", "BB", "CC"}},
		{&snapshot{Height: 2, Format: 1, Chunks: 3, Hash: []byte{1, 2}}, []types.NodeID{"AA", "BB", "CC"}},
		{&snapshot{Height: 1, Format: 2, Chunks: 5, Hash: []byte{1, 2}}, []types.NodeID{"AA", "BB", "CC"}},
	}

	// Add snapshots in reverse order, to make sure the pool enforces some order.
	for i := len(expectSnapshots) - 1; i >= 0; i-- {
		for _, peerID := range expectSnapshots[i].peers {
			_, err := pool.Add(peerID, expectSnapshots[i].snapshot)
			require.NoError(t, err)
		}
	}

	// Ranked should return the snapshots in the same order
	ranked := pool.Ranked()
	require.Len(t, ranked, len(expectSnapshots))

	for i := range ranked {
		require.Equal(t, expectSnapshots[i].snapshot, ranked[i])
	}

	// Check that best snapshots are returned in expected order
	for i := range expectSnapshots {
		snapshot := expectSnapshots[i].snapshot
		require.Equal(t, snapshot, pool.Best())
		pool.Reject(snapshot)
	}

	require.Nil(t, pool.Best())
}

func TestSnapshotPool_Reject(t *testing.T) {
	pool := newSnapshotPool()

	peerID := types.NodeID("aa")

	snapshots := []*snapshot{
		{Height: 2, Format: 2, Chunks: 1, Hash: []byte{1, 2}},
		{Height: 2, Format: 1, Chunks: 1, Hash: []byte{1, 2}},
		{Height: 1, Format: 2, Chunks: 1, Hash: []byte{1, 2}},
		{Height: 1, Format: 1, Chunks: 1, Hash: []byte{1, 2}},
	}
	for _, s := range snapshots {
		_, err := pool.Add(peerID, s)
		require.NoError(t, err)
	}

	pool.Reject(snapshots[0])
	require.Equal(t, snapshots[1:], pool.Ranked())

	added, err := pool.Add(peerID, snapshots[0])
	require.NoError(t, err)
	require.False(t, added)

	added, err = pool.Add(peerID, &snapshot{Height: 3, Format: 3, Chunks: 1, Hash: []byte{1}})
	require.NoError(t, err)
	require.True(t, added)
}

func TestSnapshotPool_RejectFormat(t *testing.T) {
	pool := newSnapshotPool()

	peerID := types.NodeID("aa")

	snapshots := []*snapshot{
		{Height: 2, Format: 2, Chunks: 1, Hash: []byte{1, 2}},
		{Height: 2, Format: 1, Chunks: 1, Hash: []byte{1, 2}},
		{Height: 1, Format: 2, Chunks: 1, Hash: []byte{1, 2}},
		{Height: 1, Format: 1, Chunks: 1, Hash: []byte{1, 2}},
	}
	for _, s := range snapshots {
		_, err := pool.Add(peerID, s)
		require.NoError(t, err)
	}

	pool.RejectFormat(1)
	require.Equal(t, []*snapshot{snapshots[0], snapshots[2]}, pool.Ranked())

	added, err := pool.Add(peerID, &snapshot{Height: 3, Format: 1, Chunks: 1, Hash: []byte{1}})
	require.NoError(t, err)
	require.False(t, added)
	require.Equal(t, []*snapshot{snapshots[0], snapshots[2]}, pool.Ranked())

	added, err = pool.Add(peerID, &snapshot{Height: 3, Format: 3, Chunks: 1, Hash: []byte{1}})
	require.NoError(t, err)
	require.True(t, added)
}

func TestSnapshotPool_RejectPeer(t *testing.T) {
	pool := newSnapshotPool()

	peerAID := types.NodeID("aa")
	peerBID := types.NodeID("bb")

	s1 := &snapshot{Height: 1, Format: 1, Chunks: 1, Hash: []byte{1}}
	s2 := &snapshot{Height: 2, Format: 1, Chunks: 1, Hash: []byte{2}}
	s3 := &snapshot{Height: 3, Format: 1, Chunks: 1, Hash: []byte{2}}

	_, err := pool.Add(peerAID, s1)
	require.NoError(t, err)

	_, err = pool.Add(peerAID, s2)
	require.NoError(t, err)

	_, err = pool.Add(peerBID, s2)
	require.NoError(t, err)

	_, err = pool.Add(peerBID, s3)
	require.NoError(t, err)

	pool.RejectPeer(peerAID)

	require.Empty(t, pool.GetPeers(s1))

	peers2 := pool.GetPeers(s2)
	require.Len(t, peers2, 1)
	require.Equal(t, peerBID, peers2[0])

	peers3 := pool.GetPeers(s2)
	require.Len(t, peers3, 1)
	require.Equal(t, peerBID, peers3[0])

	// it should no longer be possible to add the peer back
	_, err = pool.Add(peerAID, s1)
	require.NoError(t, err)
	require.Empty(t, pool.GetPeers(s1))
}

func TestSnapshotPool_RemovePeer(t *testing.T) {
	pool := newSnapshotPool()

	peerAID := types.NodeID("aa")
	peerBID := types.NodeID("bb")

	s1 := &snapshot{Height: 1, Format: 1, Chunks: 1, Hash: []byte{1}}
	s2 := &snapshot{Height: 2, Format: 1, Chunks: 1, Hash: []byte{2}}

	_, err := pool.Add(peerAID, s1)
	require.NoError(t, err)

	_, err = pool.Add(peerAID, s2)
	require.NoError(t, err)

	_, err = pool.Add(peerBID, s1)
	require.NoError(t, err)

	pool.RemovePeer(peerAID)

	peers1 := pool.GetPeers(s1)
	require.Len(t, peers1, 1)
	require.Equal(t, peerBID, peers1[0])

	peers2 := pool.GetPeers(s2)
	require.Empty(t, peers2)

	// it should still be possible to add the peer back
	_, err = pool.Add(peerAID, s1)
	require.NoError(t, err)

	peers1 = pool.GetPeers(s1)
	require.Len(t, peers1, 2)
	require.Equal(t, peerAID, peers1[0])
	require.Equal(t, peerBID, peers1[1])
}
