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
	"github.com/tendermint/tendermint/internal/test/factory"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/types"
)

type channelInternal struct {
	In    chan p2p.Envelope
	Out   chan p2p.Envelope
	Error chan p2p.PeerError
}

func testChannel(size int) (*channelInternal, p2p.Channel) {
	in := &channelInternal{
		In:    make(chan p2p.Envelope, size),
		Out:   make(chan p2p.Envelope, size),
		Error: make(chan p2p.PeerError, size),
	}
	return in, p2p.NewChannel(0, "test", in.In, in.Out, in.Error)
}

func TestDispatcherBasic(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	const numPeers = 5

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chans, ch := testChannel(100)

	d := NewDispatcher(ch)
	go handleRequests(ctx, t, d, chans.Out)

	peers := createPeerSet(numPeers)
	wg := sync.WaitGroup{}

	// make a bunch of async requests and require that the correct responses are
	// given
	for i := 0; i < numPeers; i++ {
		wg.Add(1)
		go func(height int64) {
			defer wg.Done()
			lb, err := d.LightBlock(ctx, height, peers[height-1])
			require.NoError(t, err)
			require.NotNil(t, lb)
			require.Equal(t, lb.Height, height)
		}(int64(i + 1))
	}
	wg.Wait()

	// assert that all calls were responded to
	assert.Empty(t, d.calls)
}

func TestDispatcherReturnsNoBlock(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chans, ch := testChannel(100)

	d := NewDispatcher(ch)

	peer := factory.NodeID(t, "a")

	go func() {
		<-chans.Out
		require.NoError(t, d.Respond(ctx, nil, peer))
		cancel()
	}()

	lb, err := d.LightBlock(ctx, 1, peer)
	<-ctx.Done()

	require.Nil(t, lb)
	require.NoError(t, err)
}

func TestDispatcherTimeOutWaitingOnLightBlock(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, ch := testChannel(100)
	d := NewDispatcher(ch)
	peer := factory.NodeID(t, "a")

	ctx, cancelFunc := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancelFunc()

	lb, err := d.LightBlock(ctx, 1, peer)

	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
	require.Nil(t, lb)
}

func TestDispatcherProviders(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	chainID := "test-chain"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chans, ch := testChannel(100)

	d := NewDispatcher(ch)
	go handleRequests(ctx, t, d, chans.Out)

	peers := createPeerSet(5)
	providers := make([]*BlockProvider, len(peers))
	for idx, peer := range peers {
		providers[idx] = NewBlockProvider(peer, chainID, d)
	}
	require.Len(t, providers, 5)

	for i, p := range providers {
		assert.Equal(t, string(peers[i]), p.String(), i)
		lb, err := p.LightBlock(ctx, 10)
		assert.NoError(t, err)
		assert.NotNil(t, lb)
	}
}

func TestPeerListBasic(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerList := newPeerList()
	assert.Zero(t, peerList.Len())
	numPeers := 10
	peerSet := createPeerSet(numPeers)

	for _, peer := range peerSet {
		peerList.Append(peer)
	}

	for idx, peer := range peerList.All() {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		for _, p := range peerList.All() {
			require.NotEqual(t, p, peer)
		}
		numPeers--
		require.Equal(t, numPeers, peerList.Len())
	}
}

// handleRequests is a helper function usually run in a separate go routine to
// imitate the expected responses of the reactor wired to the dispatcher
func handleRequests(ctx context.Context, t *testing.T, d *Dispatcher, ch chan p2p.Envelope) {
	t.Helper()
	for {
		select {
		case request := <-ch:
			height := request.Message.(*ssproto.LightBlockRequest).Height
			peer := request.To
			resp := mockLBResp(ctx, t, peer, int64(height), time.Now())
			block, _ := resp.block.ToProto()
			require.NoError(t, d.Respond(ctx, block, resp.peer))
		case <-ctx.Done():
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
