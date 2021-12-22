package p2p_test

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/types"
)

// Common setup for P2P tests.

var (
	chID   = p2p.ChannelID(1)
	chDesc = &p2p.ChannelDescriptor{
		ID:                  chID,
		MessageType:         &p2ptest.Message{},
		Priority:            5,
		SendQueueCapacity:   10,
		RecvMessageCapacity: 10,
	}

	selfKey  crypto.PrivKey = ed25519.GenPrivKeyFromSecret([]byte{0xf9, 0x1b, 0x08, 0xaa, 0x38, 0xee, 0x34, 0xdd})
	selfID                  = types.NodeIDFromPubKey(selfKey.PubKey())
	selfInfo                = types.NodeInfo{
		NodeID:     selfID,
		ListenAddr: "0.0.0.0:0",
		Network:    "test",
		Moniker:    string(selfID),
		Channels:   []byte{0x01, 0x02},
	}

	peerKey  crypto.PrivKey = ed25519.GenPrivKeyFromSecret([]byte{0x84, 0xd7, 0x01, 0xbf, 0x83, 0x20, 0x1c, 0xfe})
	peerID                  = types.NodeIDFromPubKey(peerKey.PubKey())
	peerInfo                = types.NodeInfo{
		NodeID:     peerID,
		ListenAddr: "0.0.0.0:0",
		Network:    "test",
		Moniker:    string(peerID),
		Channels:   []byte{0x01, 0x02},
	}
)
