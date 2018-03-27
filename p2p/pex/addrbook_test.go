package pex

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
	cmn "github.com/tendermint/tmlibs/common"
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

func deleteTempFile(fname string) {
	err := os.Remove(fname)
	if err != nil {
		panic(err)
	}
}

func TestAddrBookPickAddress(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	// 0 addresses
	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	assert.Zero(t, book.Size())

	addr := book.PickAddress(50)
	assert.Nil(t, addr, "expected no address")

	randAddrs := randNetAddressPairs(t, 1)
	addrSrc := randAddrs[0]
	book.AddAddress(addrSrc.addr, addrSrc.src)

	// pick an address when we only have new address
	addr = book.PickAddress(0)
	assert.NotNil(t, addr, "expected an address")
	addr = book.PickAddress(50)
	assert.NotNil(t, addr, "expected an address")
	addr = book.PickAddress(100)
	assert.NotNil(t, addr, "expected an address")

	// pick an address when we only have old address
	book.MarkGood(addrSrc.addr)
	addr = book.PickAddress(0)
	assert.NotNil(t, addr, "expected an address")
	addr = book.PickAddress(50)
	assert.NotNil(t, addr, "expected an address")

	// in this case, nNew==0 but we biased 100% to new, so we return nil
	addr = book.PickAddress(100)
	assert.Nil(t, addr, "did not expected an address")
}

func TestAddrBookSaveLoad(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	// 0 addresses
	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	book.saveToFile(fname)

	book = NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	book.loadFromFile(fname)

	assert.Zero(t, book.Size())

	// 100 addresses
	randAddrs := randNetAddressPairs(t, 100)

	for _, addrSrc := range randAddrs {
		book.AddAddress(addrSrc.addr, addrSrc.src)
	}

	assert.Equal(t, 100, book.Size())
	book.saveToFile(fname)

	book = NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	book.loadFromFile(fname)

	assert.Equal(t, 100, book.Size())
}

func TestAddrBookLookup(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	randAddrs := randNetAddressPairs(t, 100)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	for _, addrSrc := range randAddrs {
		addr := addrSrc.addr
		src := addrSrc.src
		book.AddAddress(addr, src)

		ka := book.addrLookup[addr.ID]
		assert.NotNil(t, ka, "Expected to find KnownAddress %v but wasn't there.", addr)

		if !(ka.Addr.Equals(addr) && ka.Src.Equals(src)) {
			t.Fatalf("KnownAddress doesn't match addr & src")
		}
	}
}

func TestAddrBookPromoteToOld(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	randAddrs := randNetAddressPairs(t, 100)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	for _, addrSrc := range randAddrs {
		book.AddAddress(addrSrc.addr, addrSrc.src)
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

	assert.Equal(t, book.Size(), 100, "expecting book size to be 100")
}

func TestAddrBookHandlesDuplicates(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())

	randAddrs := randNetAddressPairs(t, 100)

	differentSrc := randIPv4Address(t)
	for _, addrSrc := range randAddrs {
		book.AddAddress(addrSrc.addr, addrSrc.src)
		book.AddAddress(addrSrc.addr, addrSrc.src)  // duplicate
		book.AddAddress(addrSrc.addr, differentSrc) // different src
	}

	assert.Equal(t, 100, book.Size())
}

type netAddressPair struct {
	addr *p2p.NetAddress
	src  *p2p.NetAddress
}

func randNetAddressPairs(t *testing.T, n int) []netAddressPair {
	randAddrs := make([]netAddressPair, n)
	for i := 0; i < n; i++ {
		randAddrs[i] = netAddressPair{addr: randIPv4Address(t), src: randIPv4Address(t)}
	}
	return randAddrs
}

func randIPv4Address(t *testing.T) *p2p.NetAddress {
	for {
		ip := fmt.Sprintf("%v.%v.%v.%v",
			rand.Intn(254)+1,
			rand.Intn(255),
			rand.Intn(255),
			rand.Intn(255),
		)
		port := rand.Intn(65535-1) + 1
		id := p2p.ID(hex.EncodeToString(cmn.RandBytes(p2p.IDByteLength)))
		idAddr := p2p.IDAddressString(id, fmt.Sprintf("%v:%v", ip, port))
		addr, err := p2p.NewNetAddressString(idAddr)
		assert.Nil(t, err, "error generating rand network address")
		if addr.Routable() {
			return addr
		}
	}
}

func TestAddrBookRemoveAddress(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())

	addr := randIPv4Address(t)
	book.AddAddress(addr, addr)
	assert.Equal(t, 1, book.Size())

	book.RemoveAddress(addr)
	assert.Equal(t, 0, book.Size())

	nonExistingAddr := randIPv4Address(t)
	book.RemoveAddress(nonExistingAddr)
	assert.Equal(t, 0, book.Size())
}
