package p2p_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
