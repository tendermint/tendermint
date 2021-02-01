package p2p_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/p2p"
)

func TestPeerManagerOptions_Validate(t *testing.T) {
	nodeID := p2p.NodeID("00112233445566778899aabbccddeeff00112233")

	testcases := map[string]struct {
		options p2p.PeerManagerOptions
		ok      bool
	}{
		"zero options is valid": {p2p.PeerManagerOptions{}, true},

		// PersistentPeers
		"valid PersistentPeers NodeID": {p2p.PeerManagerOptions{
			PersistentPeers: []p2p.NodeID{"00112233445566778899aabbccddeeff00112233"},
		}, true},
		"invalid PersistentPeers NodeID": {p2p.PeerManagerOptions{
			PersistentPeers: []p2p.NodeID{"foo"},
		}, false},
		"uppercase PersistentPeers NodeID": {p2p.PeerManagerOptions{
			PersistentPeers: []p2p.NodeID{"00112233445566778899AABBCCDDEEFF00112233"},
		}, false},
		"PersistentPeers at MaxConnected": {p2p.PeerManagerOptions{
			PersistentPeers: []p2p.NodeID{nodeID, nodeID, nodeID},
			MaxConnected:    3,
		}, true},
		"PersistentPeers above MaxConnected": {p2p.PeerManagerOptions{
			PersistentPeers: []p2p.NodeID{nodeID, nodeID, nodeID},
			MaxConnected:    2,
		}, false},
		"PersistentPeers above MaxConnected below MaxConnectedUpgrade": {p2p.PeerManagerOptions{
			PersistentPeers:     []p2p.NodeID{nodeID, nodeID, nodeID},
			MaxConnected:        2,
			MaxConnectedUpgrade: 2,
		}, false},

		// MaxPeers
		"MaxPeers without MaxConnected": {p2p.PeerManagerOptions{
			MaxPeers: 3,
		}, false},
		"MaxPeers below MaxConnected+MaxConnectedUpgrade": {p2p.PeerManagerOptions{
			MaxPeers:            2,
			MaxConnected:        2,
			MaxConnectedUpgrade: 1,
		}, false},
		"MaxPeers at MaxConnected+MaxConnectedUpgrade": {p2p.PeerManagerOptions{
			MaxPeers:            3,
			MaxConnected:        2,
			MaxConnectedUpgrade: 1,
		}, true},

		// MaxRetryTime
		"MaxRetryTime below MinRetryTime": {p2p.PeerManagerOptions{
			MinRetryTime: 7 * time.Second,
			MaxRetryTime: 5 * time.Second,
		}, false},
		"MaxRetryTime at MinRetryTime": {p2p.PeerManagerOptions{
			MinRetryTime: 5 * time.Second,
			MaxRetryTime: 5 * time.Second,
		}, true},
		"MaxRetryTime without MinRetryTime": {p2p.PeerManagerOptions{
			MaxRetryTime: 5 * time.Second,
		}, false},

		// MaxRetryTimePersistent
		"MaxRetryTimePersistent below MinRetryTime": {p2p.PeerManagerOptions{
			MinRetryTime:           7 * time.Second,
			MaxRetryTimePersistent: 5 * time.Second,
		}, false},
		"MaxRetryTimePersistent at MinRetryTime": {p2p.PeerManagerOptions{
			MinRetryTime:           5 * time.Second,
			MaxRetryTimePersistent: 5 * time.Second,
		}, true},
		"MaxRetryTimePersistent without MinRetryTime": {p2p.PeerManagerOptions{
			MaxRetryTimePersistent: 5 * time.Second,
		}, false},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.options.Validate()
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestNewPeerManager(t *testing.T) {

	// Invalid options should error.
	_, err := p2p.NewPeerManager(dbm.NewMemDB(), p2p.PeerManagerOptions{
		PersistentPeers: []p2p.NodeID{"foo"},
	})
	require.Error(t, err)

	// Invalid database should error.
	_, err = p2p.NewPeerManager(nil, p2p.PeerManagerOptions{})
	require.Error(t, err)

	// Zero options should be valid.
	_, err = p2p.NewPeerManager(dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
}

func TestNewPeerManager_Persistence(t *testing.T) {
	aID := p2p.NodeID(strings.Repeat("a", 40))
	aAddresses := []p2p.NodeAddress{
		{Protocol: "tcp", NodeID: aID, Hostname: "127.0.0.1", Port: 26657, Path: "/path"},
		{Protocol: "memory", NodeID: aID},
	}

	bID := p2p.NodeID(strings.Repeat("b", 40))
	bAddresses := []p2p.NodeAddress{
		{Protocol: "tcp", NodeID: bID, Hostname: "b10c::1", Port: 26657, Path: "/path"},
		{Protocol: "memory", NodeID: bID},
	}

	// Create an initial peer manager and add the peers.
	db := dbm.NewMemDB()
	peerManager, err := p2p.NewPeerManager(db, p2p.PeerManagerOptions{
		PersistentPeers: []p2p.NodeID{aID},
	})
	require.NoError(t, err)
	defer peerManager.Close()

	for _, addr := range append(aAddresses, bAddresses...) {
		require.NoError(t, peerManager.Add(addr))
	}

	require.ElementsMatch(t, aAddresses, peerManager.Addresses(aID))
	require.ElementsMatch(t, bAddresses, peerManager.Addresses(bID))
	require.Equal(t, map[p2p.NodeID]p2p.PeerScore{
		aID: p2p.PeerScorePersistent,
		bID: 0,
	}, peerManager.Scores())

	peerManager.Close()

	// Creating a new peer manager with the same database should retain the
	// peers, but they should have updated scores from the new PersistentPeers
	// configuration.
	peerManager, err = p2p.NewPeerManager(db, p2p.PeerManagerOptions{
		PersistentPeers: []p2p.NodeID{bID},
	})
	require.NoError(t, err)
	defer peerManager.Close()

	require.ElementsMatch(t, aAddresses, peerManager.Addresses(aID))
	require.ElementsMatch(t, bAddresses, peerManager.Addresses(bID))
	require.Equal(t, map[p2p.NodeID]p2p.PeerScore{
		aID: 0,
		bID: p2p.PeerScorePersistent,
	}, peerManager.Scores())
}

func TestPeerManager_Add(t *testing.T) {
	aID := p2p.NodeID(strings.Repeat("a", 40))
	bID := p2p.NodeID(strings.Repeat("b", 40))
	cID := p2p.NodeID(strings.Repeat("c", 40))

	peerManager, err := p2p.NewPeerManager(dbm.NewMemDB(), p2p.PeerManagerOptions{
		PersistentPeers: []p2p.NodeID{aID, cID},
		MaxPeers:        2,
		MaxConnected:    2,
	})
	require.NoError(t, err)

	// Adding a couple of addresses should work.
	aAddresses := []p2p.NodeAddress{
		{Protocol: "tcp", NodeID: aID, Hostname: "localhost"},
		{Protocol: "memory", NodeID: aID},
	}
	for _, addr := range aAddresses {
		err = peerManager.Add(addr)
		require.NoError(t, err)
	}
	require.ElementsMatch(t, aAddresses, peerManager.Addresses(aID))

	// Adding a different peer should be fine.
	bAddress := p2p.NodeAddress{Protocol: "tcp", NodeID: bID, Hostname: "localhost"}
	err = peerManager.Add(bAddress)
	require.NoError(t, err)
	require.Equal(t, []p2p.NodeAddress{bAddress}, peerManager.Addresses(bID))
	require.ElementsMatch(t, aAddresses, peerManager.Addresses(aID))

	// Adding an existing address again should be a noop.
	err = peerManager.Add(aAddresses[0])
	require.NoError(t, err)
	require.ElementsMatch(t, aAddresses, peerManager.Addresses(aID))

	// Adding a third peer with MaxPeers=2 should cause bID, which is
	// the lowest-scored peer (not in PersistentPeers), to be removed.
	err = peerManager.Add(p2p.NodeAddress{Protocol: "tcp", NodeID: cID, Hostname: "localhost"})
	require.NoError(t, err)
	require.ElementsMatch(t, []p2p.NodeID{aID, cID}, peerManager.Peers())

	// Adding an invalid address should error.
	err = peerManager.Add(p2p.NodeAddress{Path: "foo"})
	require.Error(t, err)
}
