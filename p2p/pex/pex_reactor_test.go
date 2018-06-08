package pex

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

var (
	cfg *config.P2PConfig
)

func init() {
	cfg = config.DefaultP2PConfig()
	cfg.PexReactor = true
	cfg.AllowDuplicateIP = true
}

func TestPEXReactorBasic(t *testing.T) {
	r, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	assert.NotNil(t, r)
	assert.NotEmpty(t, r.GetChannels())
}

func TestPEXReactorAddRemovePeer(t *testing.T) {
	r, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	size := book.Size()
	peer := p2p.CreateRandomPeer(false)

	r.AddPeer(peer)
	assert.Equal(t, size+1, book.Size())

	r.RemovePeer(peer, "peer not available")

	outboundPeer := p2p.CreateRandomPeer(true)

	r.AddPeer(outboundPeer)
	assert.Equal(t, size+1, book.Size(), "outbound peers should not be added to the address book")

	r.RemovePeer(outboundPeer, "peer not available")
}

// --- FAIL: TestPEXReactorRunning (11.10s)
// 				pex_reactor_test.go:411: expected all switches to be connected to at
// 				least one peer (switches: 0 => {outbound: 1, inbound: 0}, 1 =>
// 				{outbound: 0, inbound: 1}, 2 => {outbound: 0, inbound: 0}, )
//
// EXPLANATION: peers are getting rejected because in switch#addPeer we check
// if any peer (who we already connected to) has the same IP. Even though local
// peers have different IP addresses, they all have the same underlying remote
// IP: 127.0.0.1.
//
func TestPEXReactorRunning(t *testing.T) {
	N := 3
	switches := make([]*p2p.Switch, N)

	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	books := make([]*addrBook, N)
	logger := log.TestingLogger()

	// create switches
	for i := 0; i < N; i++ {
		switches[i] = p2p.MakeSwitch(cfg, i, "testing", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch {
			books[i] = NewAddrBook(filepath.Join(dir, fmt.Sprintf("addrbook%d.json", i)), false)
			books[i].SetLogger(logger.With("pex", i))
			sw.SetAddrBook(books[i])

			sw.SetLogger(logger.With("pex", i))

			r := NewPEXReactor(books[i], &PEXReactorConfig{})
			r.SetLogger(logger.With("pex", i))
			r.SetEnsurePeersPeriod(250 * time.Millisecond)
			sw.AddReactor("pex", r)

			transport := p2p.NewMTransport(sw.NodeInfo(), *sw.NodeKey())
			p2p.MultiplexTransportMConfig(conn.DefaultMConnConfig())

			sw.SetTransport(transport)

			return sw
		})
	}

	addOtherNodeAddrToAddrBook := func(switchIndex, otherSwitchIndex int) {
		addr := switches[otherSwitchIndex].NodeInfo().NetAddress()
		books[switchIndex].AddAddress(addr, addr)
	}

	addOtherNodeAddrToAddrBook(0, 1)
	addOtherNodeAddrToAddrBook(1, 0)
	addOtherNodeAddrToAddrBook(2, 1)

	for _, sw := range switches {
		err := sw.Start() // start switch and reactors
		require.Nil(t, err)
	}

	assertPeersWithTimeout(t, switches, 10*time.Millisecond, 10*time.Second, N-1)

	// stop them
	for _, s := range switches {
		s.Stop()
	}
}

func TestPEXReactorReceive(t *testing.T) {
	r, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	peer := p2p.CreateRandomPeer(false)

	// we have to send a request to receive responses
	r.RequestAddrs(peer)

	size := book.Size()
	addrs := []*p2p.NetAddress{peer.NodeInfo().NetAddress()}
	msg := cdc.MustMarshalBinary(&pexAddrsMessage{Addrs: addrs})
	r.Receive(PexChannel, peer, msg)
	assert.Equal(t, size+1, book.Size())

	msg = cdc.MustMarshalBinary(&pexRequestMessage{})
	r.Receive(PexChannel, peer, msg) // should not panic.
}

func TestPEXReactorRequestMessageAbuse(t *testing.T) {
	r, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(r)
	sw.SetAddrBook(book)

	peer := newMockPeer()
	p2p.AddPeerToSwitch(sw, peer)
	assert.True(t, sw.Peers().Has(peer.ID()))

	id := string(peer.ID())
	msg := cdc.MustMarshalBinary(&pexRequestMessage{})

	// first time creates the entry
	r.Receive(PexChannel, peer, msg)
	assert.True(t, r.lastReceivedRequests.Has(id))
	assert.True(t, sw.Peers().Has(peer.ID()))

	// next time sets the last time value
	r.Receive(PexChannel, peer, msg)
	assert.True(t, r.lastReceivedRequests.Has(id))
	assert.True(t, sw.Peers().Has(peer.ID()))

	// third time is too many too soon - peer is removed
	r.Receive(PexChannel, peer, msg)
	assert.False(t, r.lastReceivedRequests.Has(id))
	assert.False(t, sw.Peers().Has(peer.ID()))
}

func TestPEXReactorAddrsMessageAbuse(t *testing.T) {
	r, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(r)
	sw.SetAddrBook(book)

	peer := newMockPeer()
	p2p.AddPeerToSwitch(sw, peer)
	assert.True(t, sw.Peers().Has(peer.ID()))

	id := string(peer.ID())

	// request addrs from the peer
	r.RequestAddrs(peer)
	assert.True(t, r.requestsSent.Has(id))
	assert.True(t, sw.Peers().Has(peer.ID()))

	addrs := []*p2p.NetAddress{peer.NodeInfo().NetAddress()}
	msg := cdc.MustMarshalBinary(&pexAddrsMessage{Addrs: addrs})

	// receive some addrs. should clear the request
	r.Receive(PexChannel, peer, msg)
	assert.False(t, r.requestsSent.Has(id))
	assert.True(t, sw.Peers().Has(peer.ID()))

	// receiving more addrs causes a disconnect
	r.Receive(PexChannel, peer, msg)
	assert.False(t, sw.Peers().Has(peer.ID()))
}

func TestPEXReactorUsesSeedsIfNeeded(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	// 1. create seed
	seed := p2p.MakeSwitch(
		cfg,
		0,
		"127.0.0.1",
		"123.123.123",
		func(i int, sw *p2p.Switch) *p2p.Switch {
			book := NewAddrBook(filepath.Join(dir, "addrbook0.json"), false)
			book.SetLogger(log.TestingLogger())
			sw.SetAddrBook(book)

			sw.SetLogger(log.TestingLogger())

			r := NewPEXReactor(book, &PEXReactorConfig{})
			r.SetLogger(log.TestingLogger())
			sw.AddReactor("pex", r)
			return sw
		},
	)

	transport := p2p.NewMTransport(seed.NodeInfo(), *seed.NodeKey())
	p2p.MultiplexTransportMConfig(conn.DefaultMConnConfig())
	seed.SetTransport(transport)

	require.Nil(t, seed.Start())
	defer seed.Stop()

	// 2. create usual peer with only seed configured.
	peer := p2p.MakeSwitch(
		cfg,
		1,
		"127.0.0.1",
		"123.123.123",
		func(i int, sw *p2p.Switch) *p2p.Switch {
			book := NewAddrBook(filepath.Join(dir, "addrbook1.json"), false)
			book.SetLogger(log.TestingLogger())
			sw.SetAddrBook(book)

			sw.SetLogger(log.TestingLogger())

			r := NewPEXReactor(
				book,
				&PEXReactorConfig{
					Seeds: []string{seed.NodeInfo().NetAddress().String()},
				},
			)
			r.SetLogger(log.TestingLogger())
			sw.AddReactor("pex", r)
			return sw
		},
	)
	require.Nil(t, peer.Start())
	defer peer.Stop()

	// 3. check that the peer connects to seed immediately
	assertPeersWithTimeout(t, []*p2p.Switch{peer}, 10*time.Millisecond, 3*time.Second, 1)
}

func TestPEXReactorCrawlStatus(t *testing.T) {
	pexR, book := createReactor(&PEXReactorConfig{SeedMode: true})
	defer teardownReactor(book)

	// Seed/Crawler mode uses data from the Switch
	sw := createSwitchAndAddReactors(pexR)
	sw.SetAddrBook(book)

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
	assert.Equal(t, 2, len(peerInfos))

	// TODO: test
}

func TestPEXReactorDoesNotAddPrivatePeersToAddrBook(t *testing.T) {
	peer := p2p.CreateRandomPeer(false)

	pexR, book := createReactor(&PEXReactorConfig{PrivatePeerIDs: []string{string(peer.NodeInfo().ID)}})
	defer teardownReactor(book)

	// we have to send a request to receive responses
	pexR.RequestAddrs(peer)

	size := book.Size()
	addrs := []*p2p.NetAddress{peer.NodeInfo().NetAddress()}
	msg := cdc.MustMarshalBinary(&pexAddrsMessage{Addrs: addrs})
	pexR.Receive(PexChannel, peer, msg)
	assert.Equal(t, size, book.Size())

	pexR.AddPeer(peer)
	assert.Equal(t, size, book.Size())
}

func TestPEXReactorDialPeer(t *testing.T) {
	pexR, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(pexR)
	sw.SetAddrBook(book)

	peer := newMockPeer()
	addr := peer.NodeInfo().NetAddress()

	assert.Equal(t, 0, pexR.AttemptsToDial(addr))

	// 1st unsuccessful attempt
	pexR.dialPeer(addr)

	assert.Equal(t, 1, pexR.AttemptsToDial(addr))

	// 2nd unsuccessful attempt
	pexR.dialPeer(addr)

	// must be skipped because it is too early
	assert.Equal(t, 1, pexR.AttemptsToDial(addr))

	if !testing.Short() {
		time.Sleep(3 * time.Second)

		// 3rd attempt
		pexR.dialPeer(addr)

		assert.Equal(t, 2, pexR.AttemptsToDial(addr))
	}
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

func (mp mockPeer) ID() p2p.ID         { return mp.addr.ID }
func (mp mockPeer) IsOutbound() bool   { return mp.outbound }
func (mp mockPeer) IsPersistent() bool { return mp.persistent }
func (mp mockPeer) NodeInfo() p2p.NodeInfo {
	return p2p.NodeInfo{
		ID:         mp.addr.ID,
		ListenAddr: mp.addr.DialString(),
	}
}
func (mp mockPeer) RemoteIP() net.IP              { return net.ParseIP("127.0.0.1") }
func (mp mockPeer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp mockPeer) Send(byte, []byte) bool        { return false }
func (mp mockPeer) TrySend(byte, []byte) bool     { return false }
func (mp mockPeer) Set(string, interface{})       {}
func (mp mockPeer) Get(string) interface{}        { return nil }

func assertPeersWithTimeout(
	t *testing.T,
	switches []*p2p.Switch,
	checkPeriod, timeout time.Duration,
	nPeers int,
) {
	var (
		ticker    = time.NewTicker(checkPeriod)
		remaining = timeout
	)

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
			t.Errorf(
				"expected all switches to be connected to at least one peer (switches: %s)",
				numPeersStr,
			)
			return
		}
	}
}

func createReactor(conf *PEXReactorConfig) (r *PEXReactor, book *addrBook) {
	// directory to store address book
	dir, err := ioutil.TempDir("", "pex_reactor")
	if err != nil {
		panic(err)
	}
	book = NewAddrBook(filepath.Join(dir, "addrbook.json"), true)
	book.SetLogger(log.TestingLogger())

	r = NewPEXReactor(book, conf)
	r.SetLogger(log.TestingLogger())
	return
}

func teardownReactor(book *addrBook) {
	err := os.RemoveAll(filepath.Dir(book.FilePath()))
	if err != nil {
		panic(err)
	}
}

func createSwitchAndAddReactors(reactors ...p2p.Reactor) *p2p.Switch {
	sw := p2p.MakeSwitch(cfg, 0, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch { return sw })
	sw.SetLogger(log.TestingLogger())
	for _, r := range reactors {
		sw.AddReactor(r.String(), r)
		r.SetSwitch(sw)
	}
	return sw
}
