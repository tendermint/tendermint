package pex

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/mock"
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

			return sw
		})
	}

	addOtherNodeAddrToAddrBook := func(switchIndex, otherSwitchIndex int) {
		addr := switches[otherSwitchIndex].NetAddress()
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
	addrs := []*p2p.NetAddress{peer.SocketAddr()}
	msg := cdc.MustMarshalBinaryBare(&pexAddrsMessage{Addrs: addrs})
	r.Receive(PexChannel, peer, msg)
	assert.Equal(t, size+1, book.Size())

	msg = cdc.MustMarshalBinaryBare(&pexRequestMessage{})
	r.Receive(PexChannel, peer, msg) // should not panic.
}

func TestPEXReactorRequestMessageAbuse(t *testing.T) {
	r, book := createReactor(&PEXReactorConfig{})
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(r)
	sw.SetAddrBook(book)

	peer := mock.NewPeer(nil)
	p2p.AddPeerToSwitchPeerSet(sw, peer)
	assert.True(t, sw.Peers().Has(peer.ID()))

	id := string(peer.ID())
	msg := cdc.MustMarshalBinaryBare(&pexRequestMessage{})

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

	peer := mock.NewPeer(nil)
	p2p.AddPeerToSwitchPeerSet(sw, peer)
	assert.True(t, sw.Peers().Has(peer.ID()))

	id := string(peer.ID())

	// request addrs from the peer
	r.RequestAddrs(peer)
	assert.True(t, r.requestsSent.Has(id))
	assert.True(t, sw.Peers().Has(peer.ID()))

	addrs := []*p2p.NetAddress{peer.SocketAddr()}
	msg := cdc.MustMarshalBinaryBare(&pexAddrsMessage{Addrs: addrs})

	// receive some addrs. should clear the request
	r.Receive(PexChannel, peer, msg)
	assert.False(t, r.requestsSent.Has(id))
	assert.True(t, sw.Peers().Has(peer.ID()))

	// receiving more addrs causes a disconnect
	r.Receive(PexChannel, peer, msg)
	assert.False(t, sw.Peers().Has(peer.ID()))
}

func TestCheckSeeds(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	// 1. test creating peer with no seeds works
	peerSwitch := testCreateDefaultPeer(dir, 0)
	require.Nil(t, peerSwitch.Start())
	peerSwitch.Stop()

	// 2. create seed
	seed := testCreateSeed(dir, 1, []*p2p.NetAddress{}, []*p2p.NetAddress{})

	// 3. test create peer with online seed works
	peerSwitch = testCreatePeerWithSeed(dir, 2, seed)
	require.Nil(t, peerSwitch.Start())
	peerSwitch.Stop()

	// 4. test create peer with all seeds having unresolvable DNS fails
	badPeerConfig := &PEXReactorConfig{
		Seeds: []string{"ed3dfd27bfc4af18f67a49862f04cc100696e84d@bad.network.addr:26657",
			"d824b13cb5d40fa1d8a614e089357c7eff31b670@anotherbad.network.addr:26657"},
	}
	peerSwitch = testCreatePeerWithConfig(dir, 2, badPeerConfig)
	require.Error(t, peerSwitch.Start())
	peerSwitch.Stop()

	// 5. test create peer with one good seed address succeeds
	badPeerConfig = &PEXReactorConfig{
		Seeds: []string{"ed3dfd27bfc4af18f67a49862f04cc100696e84d@bad.network.addr:26657",
			"d824b13cb5d40fa1d8a614e089357c7eff31b670@anotherbad.network.addr:26657",
			seed.NetAddress().String()},
	}
	peerSwitch = testCreatePeerWithConfig(dir, 2, badPeerConfig)
	require.Nil(t, peerSwitch.Start())
	peerSwitch.Stop()
}

func TestPEXReactorUsesSeedsIfNeeded(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	// 1. create seed
	seed := testCreateSeed(dir, 0, []*p2p.NetAddress{}, []*p2p.NetAddress{})
	require.Nil(t, seed.Start())
	defer seed.Stop()

	// 2. create usual peer with only seed configured.
	peer := testCreatePeerWithSeed(dir, 1, seed)
	require.Nil(t, peer.Start())
	defer peer.Stop()

	// 3. check that the peer connects to seed immediately
	assertPeersWithTimeout(t, []*p2p.Switch{peer}, 10*time.Millisecond, 3*time.Second, 1)
}

func TestConnectionSpeedForPeerReceivedFromSeed(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	// 1. create peer
	peerSwitch := testCreateDefaultPeer(dir, 1)
	require.Nil(t, peerSwitch.Start())
	defer peerSwitch.Stop()

	// 2. Create seed which knows about the peer
	peerAddr := peerSwitch.NetAddress()
	seed := testCreateSeed(dir, 2, []*p2p.NetAddress{peerAddr}, []*p2p.NetAddress{peerAddr})
	require.Nil(t, seed.Start())
	defer seed.Stop()

	// 3. create another peer with only seed configured.
	secondPeer := testCreatePeerWithSeed(dir, 3, seed)
	require.Nil(t, secondPeer.Start())
	defer secondPeer.Stop()

	// 4. check that the second peer connects to seed immediately
	assertPeersWithTimeout(t, []*p2p.Switch{secondPeer}, 10*time.Millisecond, 3*time.Second, 1)

	// 5. check that the second peer connects to the first peer immediately
	assertPeersWithTimeout(t, []*p2p.Switch{secondPeer}, 10*time.Millisecond, 1*time.Second, 2)
}

func TestPEXReactorSeedMode(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	pexRConfig := &PEXReactorConfig{SeedMode: true, SeedDisconnectWaitPeriod: 10 * time.Millisecond}
	pexR, book := createReactor(pexRConfig)
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(pexR)
	sw.SetAddrBook(book)
	err = sw.Start()
	require.NoError(t, err)
	defer sw.Stop()

	assert.Zero(t, sw.Peers().Size())

	peerSwitch := testCreateDefaultPeer(dir, 1)
	require.NoError(t, peerSwitch.Start())
	defer peerSwitch.Stop()

	// 1. Test crawlPeers dials the peer
	pexR.crawlPeers([]*p2p.NetAddress{peerSwitch.NetAddress()})
	assert.Equal(t, 1, sw.Peers().Size())
	assert.True(t, sw.Peers().Has(peerSwitch.NodeInfo().ID()))

	// 2. attemptDisconnects should not disconnect because of wait period
	pexR.attemptDisconnects()
	assert.Equal(t, 1, sw.Peers().Size())

	// sleep for SeedDisconnectWaitPeriod
	time.Sleep(pexRConfig.SeedDisconnectWaitPeriod + 1*time.Millisecond)

	// 3. attemptDisconnects should disconnect after wait period
	pexR.attemptDisconnects()
	assert.Equal(t, 0, sw.Peers().Size())
}

func TestPEXReactorDoesNotDisconnectFromPersistentPeerInSeedMode(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	pexRConfig := &PEXReactorConfig{SeedMode: true, SeedDisconnectWaitPeriod: 1 * time.Millisecond}
	pexR, book := createReactor(pexRConfig)
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(pexR)
	sw.SetAddrBook(book)
	err = sw.Start()
	require.NoError(t, err)
	defer sw.Stop()

	assert.Zero(t, sw.Peers().Size())

	peerSwitch := testCreateDefaultPeer(dir, 1)
	require.NoError(t, peerSwitch.Start())
	defer peerSwitch.Stop()

	err = sw.AddPersistentPeers([]string{peerSwitch.NetAddress().String()})
	require.NoError(t, err)

	// 1. Test crawlPeers dials the peer
	pexR.crawlPeers([]*p2p.NetAddress{peerSwitch.NetAddress()})
	assert.Equal(t, 1, sw.Peers().Size())
	assert.True(t, sw.Peers().Has(peerSwitch.NodeInfo().ID()))

	// sleep for SeedDisconnectWaitPeriod
	time.Sleep(pexRConfig.SeedDisconnectWaitPeriod + 1*time.Millisecond)

	// 2. attemptDisconnects should not disconnect because the peer is persistent
	pexR.attemptDisconnects()
	assert.Equal(t, 1, sw.Peers().Size())
}

func TestPEXReactorDialsPeerUpToMaxAttemptsInSeedMode(t *testing.T) {
	// directory to store address books
	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(t, err)
	defer os.RemoveAll(dir) // nolint: errcheck

	pexR, book := createReactor(&PEXReactorConfig{SeedMode: true})
	defer teardownReactor(book)

	sw := createSwitchAndAddReactors(pexR)
	sw.SetAddrBook(book)
	err = sw.Start()
	require.NoError(t, err)
	defer sw.Stop()

	peer := mock.NewPeer(nil)
	addr := peer.SocketAddr()

	err = book.AddAddress(addr, addr)
	require.NoError(t, err)

	assert.True(t, book.HasAddress(addr))
	// imitate maxAttemptsToDial reached
	pexR.attemptsToDial.Store(addr.DialString(), _attemptsToDial{maxAttemptsToDial + 1, time.Now()})
	pexR.crawlPeers([]*p2p.NetAddress{addr})
	assert.False(t, book.HasAddress(addr))
}

// connect a peer to a seed, wait a bit, then stop it.
// this should give it time to request addrs and for the seed
// to call FlushStop, and allows us to test calling Stop concurrently
// with FlushStop. Before a fix, this non-deterministically reproduced
// https://github.com/tendermint/tendermint/issues/3231.
func TestPEXReactorSeedModeFlushStop(t *testing.T) {
	N := 2
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

			config := &PEXReactorConfig{}
			if i == 0 {
				// first one is a seed node
				config = &PEXReactorConfig{SeedMode: true}
			}
			r := NewPEXReactor(books[i], config)
			r.SetLogger(logger.With("pex", i))
			r.SetEnsurePeersPeriod(250 * time.Millisecond)
			sw.AddReactor("pex", r)

			return sw
		})
	}

	for _, sw := range switches {
		err := sw.Start() // start switch and reactors
		require.Nil(t, err)
	}

	reactor := switches[0].Reactors()["pex"].(*PEXReactor)
	peerID := switches[1].NodeInfo().ID()

	err = switches[1].DialPeerWithAddress(switches[0].NetAddress())
	assert.NoError(t, err)

	// sleep up to a second while waiting for the peer to send us a message.
	// this isn't perfect since it's possible the peer sends us a msg and we FlushStop
	// before this loop catches it. but non-deterministically it works pretty well.
	for i := 0; i < 1000; i++ {
		v := reactor.lastReceivedRequests.Get(string(peerID))
		if v != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// by now the FlushStop should have happened. Try stopping the peer.
	// it should be safe to do this.
	peers := switches[0].Peers().List()
	for _, peer := range peers {
		peer.Stop()
	}

	// stop the switches
	for _, s := range switches {
		s.Stop()
	}
}

func TestPEXReactorDoesNotAddPrivatePeersToAddrBook(t *testing.T) {
	peer := p2p.CreateRandomPeer(false)

	pexR, book := createReactor(&PEXReactorConfig{})
	book.AddPrivateIDs([]string{string(peer.NodeInfo().ID())})
	defer teardownReactor(book)

	// we have to send a request to receive responses
	pexR.RequestAddrs(peer)

	size := book.Size()
	addrs := []*p2p.NetAddress{peer.SocketAddr()}
	msg := cdc.MustMarshalBinaryBare(&pexAddrsMessage{Addrs: addrs})
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

	peer := mock.NewPeer(nil)
	addr := peer.SocketAddr()

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
					break
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
				"expected all switches to be connected to at least %d peer(s) (switches: %s)",
				nPeers, numPeersStr,
			)
			return
		}
	}
}

// Creates a peer with the provided config
func testCreatePeerWithConfig(dir string, id int, config *PEXReactorConfig) *p2p.Switch {
	peer := p2p.MakeSwitch(
		cfg,
		id,
		"127.0.0.1",
		"123.123.123",
		func(i int, sw *p2p.Switch) *p2p.Switch {
			book := NewAddrBook(filepath.Join(dir, fmt.Sprintf("addrbook%d.json", id)), false)
			book.SetLogger(log.TestingLogger())
			sw.SetAddrBook(book)

			sw.SetLogger(log.TestingLogger())

			r := NewPEXReactor(
				book,
				config,
			)
			r.SetLogger(log.TestingLogger())
			sw.AddReactor("pex", r)
			return sw
		},
	)
	return peer
}

// Creates a peer with the default config
func testCreateDefaultPeer(dir string, id int) *p2p.Switch {
	return testCreatePeerWithConfig(dir, id, &PEXReactorConfig{})
}

// Creates a seed which knows about the provided addresses / source address pairs.
// Starting and stopping the seed is left to the caller
func testCreateSeed(dir string, id int, knownAddrs, srcAddrs []*p2p.NetAddress) *p2p.Switch {
	seed := p2p.MakeSwitch(
		cfg,
		id,
		"127.0.0.1",
		"123.123.123",
		func(i int, sw *p2p.Switch) *p2p.Switch {
			book := NewAddrBook(filepath.Join(dir, "addrbookSeed.json"), false)
			book.SetLogger(log.TestingLogger())
			for j := 0; j < len(knownAddrs); j++ {
				book.AddAddress(knownAddrs[j], srcAddrs[j])
				book.MarkGood(knownAddrs[j].ID)
			}
			sw.SetAddrBook(book)

			sw.SetLogger(log.TestingLogger())

			r := NewPEXReactor(book, &PEXReactorConfig{})
			r.SetLogger(log.TestingLogger())
			sw.AddReactor("pex", r)
			return sw
		},
	)
	return seed
}

// Creates a peer which knows about the provided seed.
// Starting and stopping the peer is left to the caller
func testCreatePeerWithSeed(dir string, id int, seed *p2p.Switch) *p2p.Switch {
	conf := &PEXReactorConfig{
		Seeds: []string{seed.NetAddress().String()},
	}
	return testCreatePeerWithConfig(dir, id, conf)
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
