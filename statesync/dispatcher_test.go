package statesync

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

func TestDispatcher(t *testing.T) {

	ch := make(chan p2p.Envelope)
	closeCh := make(chan struct{})
	defer close(closeCh)

	d := newDispatcher(ch)

	go handleRequests(d, ch, closeCh)

	peers := createPeerSet(5)
	for _, peer := range peers {
		d.addPeer(peer)
	}

	// make a bunch of async requests and require that the correct responses are
	// given
	for i := 1; i < 10; i++ {
		go func(height int64) {
			lb, peer, err := d.LightBlock(context.Background(), height)
			require.NoError(t, err)
			require.Equal(t, lb.Height, height)
			require.Contains(t, peers, peer)
		}(int64(i))
	}
}

func TestDispatcherProviders(t *testing.T) {
	t.TempDir()

	ch := make(chan p2p.Envelope)
	chainID := "state-sync-test"
	closeCh := make(chan struct{})
	defer close(closeCh)

	d := newDispatcher(ch)

	go handleRequests(d, ch, closeCh)

	peers := createPeerSet(5)
	for _, peer := range peers {
		d.addPeer(peer)
	}

	providers := d.Providers(chainID)
	assert.Len(t, providers, 5)
	for i, p := range providers {
		bp, ok := p.(*blockProvider)
		require.True(t, ok)
		assert.Equal(t, bp.String(), string(peers[i]))
		lb, err := p.LightBlock(context.Background(), 10)
		assert.Error(t, err)
		assert.Nil(t, lb)
	}
}


func handleRequests(d *dispatcher, ch chan p2p.Envelope, closeCh chan struct{}) {
	for {
		select {
		case request := <- ch:
			height := request.Message.(*ssproto.LightBlockRequest).Height
			peer := request.To
			resp := mocklb(peer, int64(height), time.Now())
			block, _ := resp.block.ToProto()
			d.respond(block, resp.peer)
		case <- closeCh:
			return 
		}
	}
}

func createPeerSet(num int) []p2p.NodeID {
	peers := make([]p2p.NodeID, num)
	for i := 0; i < num; i++ {
		peers[i], _ = p2p.NewNodeID(strings.Repeat(fmt.Sprintf("%d", i), 2*p2p.NodeIDByteLength))
	}
	return peers
}