package p2p

import (
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	crypto "github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tmlibs/log"
)

func createTempFileName(prefix string) string {
	f, err := ioutil.TempFile("", prefix)
	if err != nil {
		panic(err)
	}
	fname := f.Name()
	err = f.Close()
	if err != nil {
		panic(err)
	}
	return fname
}

func TestPeerBookPickPeer(t *testing.T) {
	assert := assert.New(t)
	fname := createTempFileName("addrbook_test")

	// 0 addresses
	book := NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())
	assert.Zero(book.Size())

	addr := book.PickPeer(50)
	assert.Nil(addr, "expected no address")

	randAddrs := randNetPeerPairs(t, 1)
	addrSrc := randAddrs[0]
	book.AddPeer(addrSrc.addr, addrSrc.src)

	// pick an address when we only have new address
	addr = book.PickPeer(0)
	assert.NotNil(addr, "expected an address")
	addr = book.PickPeer(50)
	assert.NotNil(addr, "expected an address")
	addr = book.PickPeer(100)
	assert.NotNil(addr, "expected an address")

	// pick an address when we only have old address
	book.MarkGood(addrSrc.addr)
	addr = book.PickPeer(0)
	assert.NotNil(addr, "expected an address")
	addr = book.PickPeer(50)
	assert.NotNil(addr, "expected an address")

	// in this case, nNew==0 but we biased 100% to new, so we return nil
	addr = book.PickPeer(100)
	assert.Nil(addr, "did not expected an address")
}

func TestPeerBookSaveLoad(t *testing.T) {
	fname := createTempFileName("addrbook_test")

	// 0 addresses
	book := NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())
	book.saveToFile(fname)

	book = NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())
	book.loadFromFile(fname)

	assert.Zero(t, book.Size())

	// 100 addresses
	randAddrs := randNetPeerPairs(t, 100)

	for _, addrSrc := range randAddrs {
		book.AddPeer(addrSrc.addr, addrSrc.src)
	}

	assert.Equal(t, 100, book.Size())
	book.saveToFile(fname)

	book = NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())
	book.loadFromFile(fname)

	assert.Equal(t, 100, book.Size())
}

func TestPeerBookLookup(t *testing.T) {
	fname := createTempFileName("addrbook_test")

	randAddrs := randNetPeerPairs(t, 100)

	book := NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())
	for _, addrSrc := range randAddrs {
		addr := addrSrc.addr
		src := addrSrc.src
		book.AddPeer(addr, src)

		ka := book.peerLookup[addr.Pretty()]
		assert.NotNil(t, ka, "Expected to find KnownPeer %v but wasn't there.", addr.Pretty())

		if !(ka.ID == addr && ka.Src == src) {
			t.Fatalf("KnownPeer doesn't match addr & src")
		}
	}
}

func TestPeerBookPromoteToOld(t *testing.T) {
	assert := assert.New(t)
	fname := createTempFileName("addrbook_test")

	randAddrs := randNetPeerPairs(t, 100)

	book := NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())
	for _, addrSrc := range randAddrs {
		book.AddPeer(addrSrc.addr, addrSrc.src)
	}

	// Attempt all addresses.
	for _, addrSrc := range randAddrs {
		book.MarkAttempt(addrSrc.addr)
	}

	// Promote half of them
	for i, addrSrc := range randAddrs {
		if i%2 == 0 {
			book.MarkGood(addrSrc.addr)
		}
	}

	// TODO: do more testing :)

	selection := book.GetSelection()
	t.Logf("selection: %v", selection)

	if len(selection) > book.Size() {
		t.Errorf("selection could not be bigger than the book")
	}

	assert.Equal(book.Size(), 100, "expecting book size to be 100")
}

func TestPeerBookHandlesDuplicates(t *testing.T) {
	fname := createTempFileName("addrbook_test")

	book := NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())

	randAddrs := randNetPeerPairs(t, 100)

	differentSrc := randPeer()
	for _, addrSrc := range randAddrs {
		book.AddPeer(addrSrc.addr, addrSrc.src)
		book.AddPeer(addrSrc.addr, addrSrc.src)  // duplicate
		book.AddPeer(addrSrc.addr, differentSrc) // different src
	}

	assert.Equal(t, 100, book.Size())
}

type peerPair struct {
	addr lpeer.ID
	src  lpeer.ID
}

func randNetPeerPairs(t *testing.T, n int) []peerPair {
	randAddrs := make([]peerPair, n)
	for i := 0; i < n; i++ {
		randAddrs[i] = peerPair{addr: randPeer(), src: randPeer()}
	}
	return randAddrs
}

var randIdSource = rand.New(rand.NewSource(time.Now().Unix()))

func randPeer() lpeer.ID {
	_, pubKey, err := crypto.GenerateEd25519Key(randIdSource)
	if err != nil {
		panic(err)
	}
	id, err := lpeer.IDFromPublicKey(pubKey)
	if err != nil {
		panic(err)
	}
	return id
}

func TestPeerBookRemovePeer(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	book := NewPeerBook(fname, true)
	book.SetLogger(log.TestingLogger())

	addr := randPeer()
	book.AddPeer(addr, addr)
	assert.Equal(t, 1, book.Size())

	book.RemovePeer(addr)
	assert.Equal(t, 0, book.Size())

	nonExistingAddr := randPeer()
	book.RemovePeer(nonExistingAddr)
	assert.Equal(t, 0, book.Size())
}
