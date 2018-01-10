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

	r := NewPEXReactor(book, false)
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

	r := NewPEXReactor(book, false)
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

			r := NewPEXReactor(book, false)
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
			if allGood {
				return
			}
		case <-time.After(timeout):
			numPeersStr := ""
			for i, s := range switches {
				outbound, inbound, _ := s.NumPeers()
				numPeersStr += fmt.Sprintf("%d => {outbound: %d, inbound: %d}, ", i, outbound, inbound)
			}
			t.Errorf("expected all switches to be connected to at least one peer (switches: %s)", numPeersStr)
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

	r := NewPEXReactor(book, false)
	r.SetLogger(log.TestingLogger())

	peer := createRandomPeer(false)

	size := book.Size()
	netAddr, _ := NewNetAddressString(peer.NodeInfo().ListenAddr)
	addrs := []*NetAddress{netAddr}
	msg := wire.BinaryBytes(struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})
	r.Receive(PexChannel, peer, msg)
	assert.Equal(size+1, book.Size())

	msg = wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})
	r.Receive(PexChannel, peer, msg)
}

func TestPEXReactorAbuseFromPeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, false)
	r.SetLogger(log.TestingLogger())
	r.SetMaxMsgCountByPeer(5)

	peer := createRandomPeer(false)

	msg := wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})
	for i := 0; i < 10; i++ {
		r.Receive(PexChannel, peer, msg)
	}

	assert.True(r.ReachedMaxMsgCountForPeer(peer.NodeInfo().ListenAddr))
}

func TestPEXReactorCrawlStatus(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir) // nolint: errcheck
	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	var r *PEXReactor
	// Seed/Crawler mode uses data from the Switch
	makeSwitch(config, 0, "127.0.0.1", "123.123.123", func(i int, sw *Switch) *Switch {
		r = NewPEXReactor(book, true)
		r.SetLogger(log.TestingLogger())
		sw.SetLogger(log.TestingLogger().With("switch", i))
		sw.AddReactor("pex", r)
		return sw
	})

	// Create a peer, and add it to the peer set
	peer := createRandomPeer(false)
	r.Switch.peers.Add(peer)
	// Add the peer address to the address book
	addr1, _ := NewNetAddressString(peer.NodeInfo().ListenAddr)
	r.book.AddAddress(addr1, addr1)
	// Add an address to the book that does not have a peer
	_, addr2 := createRoutableAddr()
	r.book.AddAddress(addr2, addr1)

	// Get the crawl status data
	status := r.getCrawlStatus()

	// Make sure it has the proper number of elements
	assert.Equal(2, len(status))

	var num int
	for _, cs := range status {
		if cs.PeerID != "" {
			num++
		}
	}
	// Check that only one has been identified as a connected peer
	assert.Equal(1, num)
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
		nodeInfo: &NodeInfo{
			ListenAddr: addr,
			RemoteAddr: netAddr.String(),
		},
		outbound: outbound,
		mconn:    &MConnection{RemoteAddress: netAddr},
	}
	p.SetLogger(log.TestingLogger().With("peer", addr))
	return p
}
