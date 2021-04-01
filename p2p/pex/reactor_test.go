package pex_test

import (
	"testing"

	// "github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/p2p/p2ptest"
)


type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors       map[p2p.NodeID]*pex.Reactor
	pexChnnels 	   map[p2p.NodeID]*p2p.Channel
	peerManager    map[p2p.NodeID]*p2p.PeerManager

	peerChans   map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates map[p2p.NodeID]*p2p.PeerUpdates

	nodes []p2p.NodeID
}


func TestPEXBasic(t *testing.T) {

}

func TestPEXSendsRequestsTooOften(t *testing.T) {

}

func TestPEXSendsResponseWithoutRequest(t *testing.T) {

}

func TestPEXSendsTooManyPeers(t *testing.T) {

}

func TestPEXSendsResponsesWithLargeDelay(t *testing.T) {
	
}

func TestPEXSmallPeerStoreInALargeNetwork(t *testing.T) {

}

func TestPEXLargePeerStoreInASmallNetwork(t *testing.T) {

}

func TestPEXWithNetworkGrowth(t *testing.T) {

}