package statesync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/types"
)

func TestDispatcherBasic(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

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

	// we should finish with as many peers as we started out with
	assert.Equal(t, 5, d.peerCount())
}

func TestDispatcherReturnsNoBlock(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	ch := make(chan p2p.Envelope, 100)
	d := newDispatcher(ch, 1*time.Second)
	peerFromSet := createPeerSet(1)[0]
	d.addPeer(peerFromSet)
	doneCh := make(chan struct{})

	go func() {
		<-ch
		require.NoError(t, d.respond(nil, peerFromSet))
		close(doneCh)
	}()

	lb, peerResult, err := d.LightBlock(context.Background(), 1)
	<-doneCh

	require.Nil(t, lb)
	require.Nil(t, err)
	require.Equal(t, peerFromSet, peerResult)
}

func TestDispatcherErrorsWhenNoPeers(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	ch := make(chan p2p.Envelope, 100)
	d := newDispatcher(ch, 1*time.Second)

	lb, peerResult, err := d.LightBlock(context.Background(), 1)

	require.Nil(t, lb)
	require.Empty(t, peerResult)
	require.Equal(t, errNoConnectedPeers, err)
}

func TestDispatcherReturnsBlockOncePeerAvailable(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	dispatcherRequestCh := make(chan p2p.Envelope, 100)
	d := newDispatcher(dispatcherRequestCh, 1*time.Second)
	peerFromSet := createPeerSet(1)[0]
	d.addPeer(peerFromSet)
	ctx := context.Background()
	wrapped, cancelFunc := context.WithCancel(ctx)

	doneCh := make(chan struct{})
	go func() {
		lb, peerResult, err := d.LightBlock(wrapped, 1)
		require.Nil(t, lb)
		require.Equal(t, peerFromSet, peerResult)
		require.Equal(t, context.Canceled, err)

		// calls to dispatcher.Lightblock write into the dispatcher's requestCh.
		// we read from the requestCh here to unblock the requestCh for future
		// calls.
		<-dispatcherRequestCh
		close(doneCh)
	}()
	cancelFunc()
	<-doneCh

	go func() {
		<-dispatcherRequestCh
		lb := &types.LightBlock{}
		asProto, err := lb.ToProto()
		require.Nil(t, err)
		err = d.respond(asProto, peerFromSet)
		require.Nil(t, err)
	}()

	lb, peerResult, err := d.LightBlock(context.Background(), 1)

	require.NotNil(t, lb)
	require.Equal(t, peerFromSet, peerResult)
	require.Nil(t, err)
}

func TestDispatcherProviders(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ch := make(chan p2p.Envelope, 100)
	chainID := "state-sync-test"
	closeCh := make(chan struct{})
	defer close(closeCh)

	d := newDispatcher(ch, 5*time.Second)

	go handleRequests(t, d, ch, closeCh)

	peers := createPeerSet(5)
	for _, peer := range peers {
		d.addPeer(peer)
	}

	providers := d.Providers(chainID)
	require.Len(t, providers, 5)
	for i, p := range providers {
		bp, ok := p.(*blockProvider)
		require.True(t, ok)
		assert.Equal(t, string(peers[i]), bp.String(), i)
		lb, err := p.LightBlock(context.Background(), 10)
		assert.Error(t, err)
		assert.Nil(t, lb)
	}
	require.Equal(t, 0, d.peerCount())
}

func TestPeerListBasic(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
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
		assert.Equal(t, peerSet[i], peerList.Pop(ctx))
	}
	assert.Equal(t, half, peerList.Len())

	// removing a peer that doesn't exist should not change the list
	peerList.Remove(types.NodeID("lp"))
	assert.Equal(t, half, peerList.Len())

	// removing a peer that exists should decrease the list size by one
	peerList.Remove(peerSet[half])
	assert.Equal(t, numPeers-half-1, peerList.Len())

	// popping the next peer should work as expected
	assert.Equal(t, peerSet[half+1], peerList.Pop(ctx))
	assert.Equal(t, numPeers-half-2, peerList.Len())

	// append the two peers back
	peerList.Append(peerSet[half])
	peerList.Append(peerSet[half+1])
	assert.Equal(t, half, peerList.Len())
}

func TestPeerListBlocksWhenEmpty(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	peerList := newPeerList()
	require.Zero(t, peerList.Len())
	doneCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		peerList.Pop(ctx)
		close(doneCh)
	}()
	select {
	case <-doneCh:
		t.Error("empty peer list should not have returned result")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEmptyPeerListReturnsWhenContextCanceled(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	peerList := newPeerList()
	require.Zero(t, peerList.Len())
	doneCh := make(chan struct{})
	ctx := context.Background()
	wrapped, cancel := context.WithCancel(ctx)
	go func() {
		peerList.Pop(wrapped)
		close(doneCh)
	}()
	select {
	case <-doneCh:
		t.Error("empty peer list should not have returned result")
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case <-doneCh:
	case <-time.After(100 * time.Millisecond):
		t.Error("peer list should have returned after context canceled")
	}
}

func TestPeerListConcurrent(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	peerList := newPeerList()
	numPeers := 10

	wg := sync.WaitGroup{}
	// we run a set of goroutines requesting the next peer in the list. As the
	// peer list hasn't been populated each these go routines should block
	for i := 0; i < numPeers/2; i++ {
		go func() {
			_ = peerList.Pop(ctx)
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
			_ = peerList.Pop(ctx)
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

func TestPeerListRemove(t *testing.T) {
	peerList := newPeerList()
	numPeers := 10

	peerSet := createPeerSet(numPeers)
	for _, peer := range peerSet {
		peerList.Append(peer)
	}

	for _, peer := range peerSet {
		peerList.Remove(peer)
		for _, p := range peerList.Peers() {
			require.NotEqual(t, p, peer)
		}
		numPeers--
		require.Equal(t, numPeers, peerList.Len())
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
