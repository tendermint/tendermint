package pex

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/wire"
)

var (
	config *cfg.P2PConfig
)

func init() {
	config = cfg.DefaultP2PConfig()
	config.PexReactor = true
}

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
	peer := p2p.CreateRandomPeer(false)

	r.AddPeer(peer)
	assert.Equal(size+1, book.Size())

	r.RemovePeer(peer, "peer not available")
	assert.Equal(size+1, book.Size())

	outboundPeer := p2p.CreateRandomPeer(true)

	r.AddPeer(outboundPeer)
	assert.Equal(size+1, book.Size(), "outbound peers should not be added to the address book")

	r.RemovePeer(outboundPeer, "peer not available")
	assert.Equal(size+1, book.Size())
}

func TestPEXReactorRunning(t *testing.T) {
	N := 3
	switches := make([]*p2p.Switch, N)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	// create switches
	for i := 0; i < N; i++ {
		switches[i] = p2p.MakeSwitch(config, i, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch {
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
		addr, _ := p2p.NewNetAddressString(s.NodeInfo().ListenAddr)
		book.AddAddress(addr, addr)
		s.AddListener(p2p.NewDefaultListener("tcp", s.NodeInfo().ListenAddr, true, log.TestingLogger()))
	}

	// start switches
	for _, s := range switches {
		err := s.Start() // start switch and reactors
		require.Nil(t, err)
	}

	assertPeersWithTimeout(t, switches, 10*time.Millisecond, 10*time.Second, N-1)

	// stop them
	for _, s := range switches {
		s.Stop()
	}
}

func assertPeersWithTimeout(t *testing.T, switches []*p2p.Switch, checkPeriod, timeout time.Duration, nPeers int) {
	ticker := time.NewTicker(checkPeriod)
	remaining := timeout
	for {
		select {
		case <-ticker.C:
			// check peers are connected
			allGood := true
			for _, s := range switches {
				outbound, inbound, _ := s.NumPeers()
				if outbound+inbound < nPeers {
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

	peer := p2p.CreateRandomPeer(false)

	// we have to send a request to receive responses
	r.RequestAddrs(peer)

	size := book.Size()
	addrs := []*p2p.NetAddress{peer.NodeInfo().NetAddress()}
	msg, err := wire.MarshalBinary(&pexAddrsMessage{Addrs: addrs})
	require.NoError(err)
	r.Receive(PexChannel, peer, msg)
	assert.Equal(size+1, book.Size())

	msg, err = wire.MarshalBinary(&pexRequestMessage{})
	require.NoError(err)
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
	sw := p2p.MakeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch { return sw })
	sw.SetLogger(log.TestingLogger())
	sw.AddReactor("PEX", r)
	r.SetSwitch(sw)
	r.SetLogger(log.TestingLogger())

	peer := newMockPeer()
	p2p.AddPeerToSwitch(sw, peer)
	assert.True(sw.Peers().Has(peer.ID()))

	id := string(peer.ID())
	msg, err := wire.MarshalBinary(&pexRequestMessage{})
	require.NoError(err)

	// first time creates the entry
	r.Receive(PexChannel, peer, msg)
	assert.True(r.lastReceivedRequests.Has(id))
	assert.True(sw.Peers().Has(peer.ID()))

	// next time sets the last time value
	r.Receive(PexChannel, peer, msg)
	assert.True(r.lastReceivedRequests.Has(id))
	assert.True(sw.Peers().Has(peer.ID()))

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
	sw := p2p.MakeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch { return sw })
	sw.SetLogger(log.TestingLogger())
	sw.AddReactor("PEX", r)
	r.SetSwitch(sw)
	r.SetLogger(log.TestingLogger())

	peer := newMockPeer()
	p2p.AddPeerToSwitch(sw, peer)
	assert.True(sw.Peers().Has(peer.ID()))

	id := string(peer.ID())

	// request addrs from the peer
	r.RequestAddrs(peer)
	assert.True(r.requestsSent.Has(id))
	assert.True(sw.Peers().Has(peer.ID()))

	addrs := []*p2p.NetAddress{peer.NodeInfo().NetAddress()}
	msg, err := wire.MarshalBinary(&pexAddrsMessage{Addrs: addrs})
	require.NoError(err)

	// receive some addrs. should clear the request
	r.Receive(PexChannel, peer, msg)
	assert.False(r.requestsSent.Has(id))
	assert.True(sw.Peers().Has(peer.ID()))

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
	seed := p2p.MakeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch {
		sw.SetLogger(log.TestingLogger())

		r := NewPEXReactor(book, &PEXReactorConfig{})
		r.SetLogger(log.TestingLogger())
		r.SetEnsurePeersPeriod(250 * time.Millisecond)
		sw.AddReactor("pex", r)
		return sw
	})
	seed.AddListener(p2p.NewDefaultListener("tcp", seed.NodeInfo().ListenAddr, true, log.TestingLogger()))
	err = seed.Start()
	require.Nil(t, err)
	defer seed.Stop()

	// 2. create usual peer
	sw := p2p.MakeSwitch(config, 1, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch {
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
	assertPeersWithTimeout(t, []*p2p.Switch{sw}, 10*time.Millisecond, 10*time.Second, 1)
}

func TestPEXReactorCrawlStatus(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	pexR := NewPEXReactor(book, &PEXReactorConfig{SeedMode: true})
	// Seed/Crawler mode uses data from the Switch
	p2p.MakeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch {
		pexR.SetLogger(log.TestingLogger())
		sw.SetLogger(log.TestingLogger().With("switch", i))
		sw.AddReactor("pex", pexR)
		return sw
	})

	// Create a peer, add it to the peer set and the addrbook.
	peer := p2p.CreateRandomPeer(false)
	p2p.AddPeerToSwitch(pexR.Switch, peer)
	addr1 := peer.NodeInfo().NetAddress()
	pexR.book.AddAddress(addr1, addr1)

	// Add a non-connected address to the book.
	_, addr2 := p2p.CreateRoutableAddr()
	pexR.book.AddAddress(addr2, addr1)

	// Get some peerInfos to crawl
	peerInfos := pexR.getPeersToCrawl()

	// Make sure it has the proper number of elements
	assert.Equal(2, len(peerInfos))

	// TODO: test
}

type mockPeer struct {
	*cmn.BaseService
	pubKey               crypto.PubKey
	addr                 *p2p.NetAddress
	outbound, persistent bool
}

func newMockPeer() mockPeer {
	_, netAddr := p2p.CreateRoutableAddr()
	mp := mockPeer{
		addr:   netAddr,
		pubKey: crypto.GenPrivKeyEd25519().PubKey(),
	}
	mp.BaseService = cmn.NewBaseService(nil, "MockPeer", mp)
	mp.Start()
	return mp
}

func (mp mockPeer) ID() p2p.ID         { return p2p.PubKeyToID(mp.pubKey) }
func (mp mockPeer) IsOutbound() bool   { return mp.outbound }
func (mp mockPeer) IsPersistent() bool { return mp.persistent }
func (mp mockPeer) NodeInfo() p2p.NodeInfo {
	return p2p.NodeInfo{
		PubKey:     mp.pubKey,
		ListenAddr: mp.addr.DialString(),
	}
}
func (mp mockPeer) Status() conn.ConnectionStatus  { return conn.ConnectionStatus{} }
func (mp mockPeer) Send(byte, interface{}) bool    { return false }
func (mp mockPeer) TrySend(byte, interface{}) bool { return false }
func (mp mockPeer) Set(string, interface{})        {}
func (mp mockPeer) Get(string) interface{}         { return nil }
