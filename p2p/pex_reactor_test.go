package p2p

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

func TestPEXReactorBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, &PEXReactorConfig{})
	r.SetLogger(log.TestingLogger())

	assert.NotNil(r)
	assert.NotEmpty(r.GetChannels())
}

func TestPEXReactorAddRemovePeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, &PEXReactorConfig{})
	r.SetLogger(log.TestingLogger())

	size := book.Size()
	peer := createRandomPeer(false)

	r.AddPeer(peer)
	assert.Equal(size+1, book.Size())

	r.RemovePeer(peer, "peer not available")
	assert.Equal(size+1, book.Size())

	outboundPeer := createRandomPeer(true)

	r.AddPeer(outboundPeer)
	assert.Equal(size+1, book.Size(), "outbound peers should not be added to the address book")

	r.RemovePeer(outboundPeer, "peer not available")
	assert.Equal(size+1, book.Size())
}

func TestPEXReactorRunning(t *testing.T) {
	N := 3
	switches := make([]*Switch, N)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	// create switches
	for i := 0; i < N; i++ {
		switches[i] = makeSwitch(config, i, "127.0.0.1", "123.123.123", func(i int, sw *Switch) *Switch {
			sw.SetLogger(log.TestingLogger().With("switch", i))

			r := NewPEXReactor(book, &PEXReactorConfig{})
			r.SetLogger(log.TestingLogger())
			r.SetEnsurePeersPeriod(250 * time.Millisecond)
			sw.AddReactor("pex", r)
			return sw
		})
	}

	// fill the address book and add listeners
	for _, s := range switches {
		addr, _ := NewNetAddressString(s.NodeInfo().ListenAddr)
		book.AddAddress(addr, addr)
		s.AddListener(NewDefaultListener("tcp", s.NodeInfo().ListenAddr, true, log.TestingLogger()))
	}

	// start switches
	for _, s := range switches {
		err := s.Start() // start switch and reactors
		require.Nil(t, err)
	}

	assertSomePeersWithTimeout(t, switches, 10*time.Millisecond, 10*time.Second)

	// stop them
	for _, s := range switches {
		s.Stop()
	}
}

func assertSomePeersWithTimeout(t *testing.T, switches []*Switch, checkPeriod, timeout time.Duration) {
	ticker := time.NewTicker(checkPeriod)
	remaining := timeout
	for {
		select {
		case <-ticker.C:
			// check peers are connected
			allGood := true
			for _, s := range switches {
				outbound, inbound, _ := s.NumPeers()
				if outbound+inbound == 0 {
					allGood = false
				}
			}
			remaining -= checkPeriod
			if remaining < 0 {
				remaining = 0
			}
			if allGood {
				return
			}
		case <-time.After(remaining):
			numPeersStr := ""
			for i, s := range switches {
				outbound, inbound, _ := s.NumPeers()
				numPeersStr += fmt.Sprintf("%d => {outbound: %d, inbound: %d}, ", i, outbound, inbound)
			}
			t.Errorf("expected all switches to be connected to at least one peer (switches: %s)", numPeersStr)
			return
		}
	}
}

func TestPEXReactorReceive(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, &PEXReactorConfig{})
	r.SetLogger(log.TestingLogger())

	peer := createRandomPeer(false)

	// we have to send a request to receive responses
	r.RequestPEX(peer)

	size := book.Size()
	addrs := []*NetAddress{peer.NodeInfo().NetAddress()}
	msg := wire.BinaryBytes(struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})
	r.Receive(PexChannel, peer, msg)
	assert.Equal(size+1, book.Size())

	msg = wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})
	r.Receive(PexChannel, peer, msg)
}

func TestPEXReactorRequestMessageAbuse(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, &PEXReactorConfig{})
	sw := makeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *Switch) *Switch { return sw })
	sw.SetLogger(log.TestingLogger())
	sw.AddReactor("PEX", r)
	r.SetSwitch(sw)
	r.SetLogger(log.TestingLogger())

	peer := newMockPeer()

	id := string(peer.ID())
	msg := wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})

	// first time creates the entry
	r.Receive(PexChannel, peer, msg)
	assert.True(r.lastReceivedRequests.Has(id))

	// next time sets the last time value
	r.Receive(PexChannel, peer, msg)
	assert.True(r.lastReceivedRequests.Has(id))

	// third time is too many too soon - peer is removed
	r.Receive(PexChannel, peer, msg)
	assert.False(r.lastReceivedRequests.Has(id))
	assert.False(sw.Peers().Has(peer.ID()))

}

func TestPEXReactorAddrsMessageAbuse(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, &PEXReactorConfig{})
	sw := makeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *Switch) *Switch { return sw })
	sw.SetLogger(log.TestingLogger())
	sw.AddReactor("PEX", r)
	r.SetSwitch(sw)
	r.SetLogger(log.TestingLogger())

	peer := newMockPeer()

	id := string(peer.ID())

	// request addrs from the peer
	r.RequestPEX(peer)
	assert.True(r.requestsSent.Has(id))

	addrs := []*NetAddress{peer.NodeInfo().NetAddress()}
	msg := wire.BinaryBytes(struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})

	// receive some addrs. should clear the request
	r.Receive(PexChannel, peer, msg)
	assert.False(r.requestsSent.Has(id))

	// receiving more addrs causes a disconnect
	r.Receive(PexChannel, peer, msg)
	assert.False(sw.Peers().Has(peer.ID()))
}

func TestPEXReactorUsesSeedsIfNeeded(t *testing.T) {
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	// 1. create seed
	seed := makeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *Switch) *Switch {
		sw.SetLogger(log.TestingLogger())

		r := NewPEXReactor(book, &PEXReactorConfig{})
		r.SetLogger(log.TestingLogger())
		r.SetEnsurePeersPeriod(250 * time.Millisecond)
		sw.AddReactor("pex", r)
		return sw
	})
	seed.AddListener(NewDefaultListener("tcp", seed.NodeInfo().ListenAddr, true, log.TestingLogger()))
	err = seed.Start()
	require.Nil(t, err)
	defer seed.Stop()

	// 2. create usual peer
	sw := makeSwitch(config, 1, "127.0.0.1", "123.123.123", func(i int, sw *Switch) *Switch {
		sw.SetLogger(log.TestingLogger())

		r := NewPEXReactor(book, &PEXReactorConfig{Seeds: []string{seed.NodeInfo().ListenAddr}})
		r.SetLogger(log.TestingLogger())
		r.SetEnsurePeersPeriod(250 * time.Millisecond)
		sw.AddReactor("pex", r)
		return sw
	})
	err = sw.Start()
	require.Nil(t, err)
	defer sw.Stop()

	// 3. check that peer at least connects to seed
	assertSomePeersWithTimeout(t, []*Switch{sw}, 10*time.Millisecond, 10*time.Second)
}

func createRoutableAddr() (addr string, netAddr *NetAddress) {
	for {
		addr = cmn.Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256)
		netAddr, _ = NewNetAddressString(addr)
		if netAddr.Routable() {
			break
		}
	}
	return
}

func createRandomPeer(outbound bool) *peer {
	addr, netAddr := createRoutableAddr()
	p := &peer{
		nodeInfo: NodeInfo{
			ListenAddr: netAddr.String(),
			PubKey:     crypto.GenPrivKeyEd25519().Wrap().PubKey(),
		},
		outbound: outbound,
		mconn:    &MConnection{},
	}
	p.SetLogger(log.TestingLogger().With("peer", addr))
	return p
}

type mockPeer struct {
	*cmn.BaseService
	pubKey               crypto.PubKey
	addr                 *NetAddress
	outbound, persistent bool
}

func newMockPeer() mockPeer {
	_, netAddr := createRoutableAddr()
	mp := mockPeer{
		addr:   netAddr,
		pubKey: crypto.GenPrivKeyEd25519().Wrap().PubKey(),
	}
	mp.BaseService = cmn.NewBaseService(nil, "MockPeer", mp)
	mp.Start()
	return mp
}

func (mp mockPeer) ID() ID             { return PubKeyToID(mp.pubKey) }
func (mp mockPeer) IsOutbound() bool   { return mp.outbound }
func (mp mockPeer) IsPersistent() bool { return mp.persistent }
func (mp mockPeer) NodeInfo() NodeInfo {
	return NodeInfo{
		PubKey:     mp.pubKey,
		ListenAddr: mp.addr.DialString(),
	}
}
func (mp mockPeer) Status() ConnectionStatus       { return ConnectionStatus{} }
func (mp mockPeer) Send(byte, interface{}) bool    { return false }
func (mp mockPeer) TrySend(byte, interface{}) bool { return false }
func (mp mockPeer) Set(string, interface{})        {}
func (mp mockPeer) Get(string) interface{}         { return nil }
