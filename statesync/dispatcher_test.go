package statesync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

func TestDispatcherBasic(t *testing.T) {

	ch := make(chan p2p.Envelope, 100)
	closeCh := make(chan struct{})
	defer close(closeCh)

	d := newDispatcher(ch, 1*time.Second)

	go handleRequests(t, d, ch, closeCh)

	peers := createPeerSet(5)
	for _, peer := range peers {
		d.addPeer(peer)
	}

	wg := sync.WaitGroup{}

	// make a bunch of async requests and require that the correct responses are
	// given
	for i := 1; i < 10; i++ {
		wg.Add(1)
		go func(height int64) {
			lb, peer, err := d.LightBlock(context.Background(), height)
			require.NoError(t, err)
			require.NotNil(t, lb)
			require.Equal(t, lb.Height, height)
			require.Contains(t, peers, peer)
			wg.Done()
		}(int64(i))
	}
	wg.Wait()
}

func TestDispatcherProviders(t *testing.T) {

	ch := make(chan p2p.Envelope, 100)
	chainID := "state-sync-test"
	closeCh := make(chan struct{})
	defer close(closeCh)

	d := newDispatcher(ch, 1*time.Second)

	go handleRequests(t, d, ch, closeCh)

	peers := createPeerSet(5)
	for _, peer := range peers {
		d.addPeer(peer)
	}

	providers := d.Providers(chainID, 5*time.Second)
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

func TestPeerListBasic(t *testing.T) {
	peerList := newPeerList()
	assert.Zero(t, peerList.Len())
	numPeers := 10
	peerSet := createPeerSet(numPeers)

	for _, peer := range peerSet {
		peerList.Append(peer)
	}

	for idx, peer := range peerList.Peers() {
		assert.Equal(t, peer, peerSet[idx])
	}

	assert.Equal(t, numPeers, peerList.Len())

	half := numPeers / 2
	for i := 0; i < half; i++ {
		assert.Equal(t, peerSet[i], peerList.Pop())
	}
	assert.Equal(t, half, peerList.Len())

	peerList.Remove(p2p.NodeID("lp"))
	assert.Equal(t, half, peerList.Len())

	peerList.Remove(peerSet[half])
	half++
	assert.Equal(t, peerSet[half], peerList.Pop())

}

func TestPeerListConcurrent(t *testing.T) {
	peerList := newPeerList()
	numPeers := 10

	wg := sync.WaitGroup{}
	for i := 0; i < numPeers/2; i++ {
		go func() {
			_ = peerList.Pop()
			wg.Done()
		}()
	}

	for _, peer := range createPeerSet(numPeers) {
		wg.Add(1)
		peerList.Append(peer)
	}

	for i := 0; i < numPeers/2; i++ {
		go func() {
			_ = peerList.Pop()
			wg.Done()
		}()
	}

	wg.Wait()

}

func handleRequests(t *testing.T, d *dispatcher, ch chan p2p.Envelope, closeCh chan struct{}) {
	t.Helper()
	for {
		select {
		case request := <-ch:
			height := request.Message.(*ssproto.LightBlockRequest).Height
			peer := request.To
			resp := mockLBResp(t, peer, int64(height), time.Now())
			block, _ := resp.block.ToProto()
			require.NoError(t, d.respond(block, resp.peer))
		case <-closeCh:
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
