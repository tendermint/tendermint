package p2ptest

import "github.com/tendermint/tendermint/p2p"

// TestNetwork sets up an in-memory network that can be used for high-level P2P
// testing.
type TestNetwork struct {
	Router      *p2p.Router
	PeerManager *p2p.PeerManager
}
