package p2p

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/tendermint/go-common"
)

func TestPEXReactorBasic(t *testing.T) {
	book := NewAddrBook(createTempFileName("addrbook"), true)
	r := NewPEXReactor(book)

	assert.NotNil(t, r)
	assert.NotEmpty(t, r.GetChannels())
}

func TestPEXReactorAddRemovePeer(t *testing.T) {
	book := NewAddrBook(createTempFileName("addrbook"), true)
	r := NewPEXReactor(book)

	size := book.Size()
	peer := createRandomPeer(false)

	r.AddPeer(peer)
	assert.Equal(t, size+1, book.Size())

	r.RemovePeer(peer, "peer not available")
	assert.Equal(t, size, book.Size())

	outboundPeer := createRandomPeer(true)

	r.AddPeer(outboundPeer)
	assert.Equal(t, size, book.Size(), "size must not change")

	r.RemovePeer(outboundPeer, "peer not available")
	assert.Equal(t, size, book.Size(), "size must not change")
}

func TestPEXReactorRunning(t *testing.T) {
	N := 3
	switches := make([]*Switch, N)

	book := NewAddrBook(createTempFileName("addrbook"), false)

	// create switches
	for i := 0; i < N; i++ {
		switches[i] = makeSwitch(i, "172.17.0.2", "123.123.123", func(i int, sw *Switch) *Switch {
			r := NewPEXReactor(book)
			r.SetEnsurePeersPeriod(250 * time.Millisecond)
			sw.AddReactor("pex", r)
			return sw
		})
	}

	// fill the address book and add listeners
	for _, s := range switches {
		addr := NewNetAddressString(s.NodeInfo().ListenAddr)
		book.AddAddress(addr, addr)
		s.AddListener(NewDefaultListener("tcp", s.NodeInfo().ListenAddr, true))
	}

	// start switches
	for _, s := range switches {
		_, err := s.Start() // start switch and reactors
		require.Nil(t, err)
	}

	time.Sleep(1 * time.Second)

	// check peers are connected after some time
	for _, s := range switches {
		outbound, inbound, _ := s.NumPeers()
		if outbound+inbound == 0 {
			t.Errorf("%v expected to be connected to at least one peer", s.NodeInfo().ListenAddr)
		}
	}

	// stop them
	for _, s := range switches {
		s.Stop()
	}
}

func createRandomPeer(outbound bool) *Peer {
	return &Peer{
		Key: RandStr(12),
		NodeInfo: &NodeInfo{
			RemoteAddr: Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
			ListenAddr: Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
		},
		outbound: outbound,
	}
}
