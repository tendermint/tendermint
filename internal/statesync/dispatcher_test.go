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

	"github.com/tendermint/tendermint/internal/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/types"
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
			defer wg.Done()
			lb, peer, err := d.LightBlock(context.Background(), height)
			require.NoError(t, err)
			require.NotNil(t, lb)
			require.Equal(t, lb.Height, height)
			require.Contains(t, peers, peer)
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
	require.Len(t, providers, 5)
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

	peerList.Remove(types.NodeID("lp"))
	assert.Equal(t, half, peerList.Len())

	peerList.Remove(peerSet[half])
	half++
	assert.Equal(t, peerSet[half], peerList.Pop())

}

func TestPeerListConcurrent(t *testing.T) {
	peerList := newPeerList()
	numPeers := 10

	wg := sync.WaitGroup{}
	// we run a set of goroutines requesting the next peer in the list. As the
	// peer list hasn't been populated each these go routines should block
	for i := 0; i < numPeers/2; i++ {
		go func() {
			_ = peerList.Pop()
			wg.Done()
		}()
	}

	// now we add the peers to the list, this should allow the previously
	// blocked go routines to unblock
	for _, peer := range createPeerSet(numPeers) {
		wg.Add(1)
		peerList.Append(peer)
	}

	// we request the second half of the peer set
	for i := 0; i < numPeers/2; i++ {
		go func() {
			_ = peerList.Pop()
			wg.Done()
		}()
	}

	// we use a context with cancel and a separate go routine to wait for all
	// the other goroutines to close.
	ctx, cancel := context.WithCancel(context.Background())
	go func() { wg.Wait(); cancel() }()

	select {
	case <-time.After(time.Second):
		// not all of the blocked go routines waiting on peers have closed after
		// one second. This likely means the list got blocked.
		t.Failed()
	case <-ctx.Done():
		// there should be no peers remaining
		require.Equal(t, 0, peerList.Len())
	}
}

// handleRequests is a helper function usually run in a separate go routine to
// imitate the expected responses of the reactor wired to the dispatcher
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

func createPeerSet(num int) []types.NodeID {
	peers := make([]types.NodeID, num)
	for i := 0; i < num; i++ {
		peers[i], _ = types.NewNodeID(strings.Repeat(fmt.Sprintf("%d", i), 2*types.NodeIDByteLength))
	}
	return peers
}
