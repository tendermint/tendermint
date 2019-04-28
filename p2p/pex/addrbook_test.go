package pex

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

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
	book.MarkGood(addrSrc.addr.ID)
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
			book.MarkGood(addrSrc.addr.ID)
		}
	}

	// TODO: do more testing :)

	selection := book.GetSelection()
	t.Logf("selection: %v", selection)

	if len(selection) > book.Size() {
		t.Errorf("selection could not be bigger than the book")
	}

	selection = book.GetSelectionWithBias(30)
	t.Logf("selection: %v", selection)

	if len(selection) > book.Size() {
		t.Errorf("selection with bias could not be bigger than the book")
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
			cmn.RandIntn(254)+1,
			cmn.RandIntn(255),
			cmn.RandIntn(255),
			cmn.RandIntn(255),
		)
		port := cmn.RandIntn(65535-1) + 1
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

func TestAddrBookGetSelectionWithOneMarkedGood(t *testing.T) {
	// create a book with 10 addresses, 1 good/old and 9 new
	book, fname := createAddrBookWithMOldAndNNewAddrs(t, 1, 9)
	defer deleteTempFile(fname)

	addrs := book.GetSelectionWithBias(biasToSelectNewPeers)
	assert.NotNil(t, addrs)
	assertMOldAndNNewAddrsInSelection(t, 1, 9, addrs, book)
}

func TestAddrBookGetSelectionWithOneNotMarkedGood(t *testing.T) {
	// create a book with 10 addresses, 9 good/old and 1 new
	book, fname := createAddrBookWithMOldAndNNewAddrs(t, 9, 1)
	defer deleteTempFile(fname)

	addrs := book.GetSelectionWithBias(biasToSelectNewPeers)
	assert.NotNil(t, addrs)
	assertMOldAndNNewAddrsInSelection(t, 9, 1, addrs, book)
}

func TestAddrBookGetSelectionReturnsNilWhenAddrBookIsEmpty(t *testing.T) {
	book, fname := createAddrBookWithMOldAndNNewAddrs(t, 0, 0)
	defer deleteTempFile(fname)

	addrs := book.GetSelectionWithBias(biasToSelectNewPeers)
	assert.Nil(t, addrs)
}

func TestAddrBookGetSelection(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())

	// 1) empty book
	assert.Empty(t, book.GetSelection())

	// 2) add one address
	addr := randIPv4Address(t)
	book.AddAddress(addr, addr)

	assert.Equal(t, 1, len(book.GetSelection()))
	assert.Equal(t, addr, book.GetSelection()[0])

	// 3) add a bunch of addresses
	randAddrs := randNetAddressPairs(t, 100)
	for _, addrSrc := range randAddrs {
		book.AddAddress(addrSrc.addr, addrSrc.src)
	}

	// check there is no duplicates
	addrs := make(map[string]*p2p.NetAddress)
	selection := book.GetSelection()
	for _, addr := range selection {
		if dup, ok := addrs[addr.String()]; ok {
			t.Fatalf("selection %v contains duplicates %v", selection, dup)
		}
		addrs[addr.String()] = addr
	}

	if len(selection) > book.Size() {
		t.Errorf("selection %v could not be bigger than the book", selection)
	}
}

func TestAddrBookGetSelectionWithBias(t *testing.T) {
	const biasTowardsNewAddrs = 30

	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())

	// 1) empty book
	selection := book.GetSelectionWithBias(biasTowardsNewAddrs)
	assert.Empty(t, selection)

	// 2) add one address
	addr := randIPv4Address(t)
	book.AddAddress(addr, addr)

	selection = book.GetSelectionWithBias(biasTowardsNewAddrs)
	assert.Equal(t, 1, len(selection))
	assert.Equal(t, addr, selection[0])

	// 3) add a bunch of addresses
	randAddrs := randNetAddressPairs(t, 100)
	for _, addrSrc := range randAddrs {
		book.AddAddress(addrSrc.addr, addrSrc.src)
	}

	// check there is no duplicates
	addrs := make(map[string]*p2p.NetAddress)
	selection = book.GetSelectionWithBias(biasTowardsNewAddrs)
	for _, addr := range selection {
		if dup, ok := addrs[addr.String()]; ok {
			t.Fatalf("selection %v contains duplicates %v", selection, dup)
		}
		addrs[addr.String()] = addr
	}

	if len(selection) > book.Size() {
		t.Fatalf("selection %v could not be bigger than the book", selection)
	}

	// 4) mark 80% of the addresses as good
	randAddrsLen := len(randAddrs)
	for i, addrSrc := range randAddrs {
		if int((float64(i)/float64(randAddrsLen))*100) >= 20 {
			book.MarkGood(addrSrc.addr.ID)
		}
	}

	selection = book.GetSelectionWithBias(biasTowardsNewAddrs)

	// check that ~70% of addresses returned are good
	good := 0
	for _, addr := range selection {
		if book.IsGood(addr) {
			good++
		}
	}

	got, expected := int((float64(good)/float64(len(selection)))*100), (100 - biasTowardsNewAddrs)

	// compute some slack to protect against small differences due to rounding:
	slack := int(math.Round(float64(100) / float64(len(selection))))
	if got > expected+slack {
		t.Fatalf("got more good peers (%% got: %d, %% expected: %d, number of good addrs: %d, total: %d)", got, expected, good, len(selection))
	}
	if got < expected-slack {
		t.Fatalf("got fewer good peers (%% got: %d, %% expected: %d, number of good addrs: %d, total: %d)", got, expected, good, len(selection))
	}
}

func TestAddrBookHasAddress(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	addr := randIPv4Address(t)
	book.AddAddress(addr, addr)

	assert.True(t, book.HasAddress(addr))

	book.RemoveAddress(addr)

	assert.False(t, book.HasAddress(addr))
}

func testCreatePrivateAddrs(t *testing.T, numAddrs int) ([]*p2p.NetAddress, []string) {
	addrs := make([]*p2p.NetAddress, numAddrs)
	for i := 0; i < numAddrs; i++ {
		addrs[i] = randIPv4Address(t)
	}

	private := make([]string, numAddrs)
	for i, addr := range addrs {
		private[i] = string(addr.ID)
	}
	return addrs, private
}

func TestAddrBookEmpty(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	// Check that empty book is empty
	require.True(t, book.Empty())
	// Check that book with our address is empty
	book.AddOurAddress(randIPv4Address(t))
	require.True(t, book.Empty())
	// Check that book with private addrs is empty
	_, privateIds := testCreatePrivateAddrs(t, 5)
	book.AddPrivateIDs(privateIds)
	require.True(t, book.Empty())

	// Check that book with address is not empty
	book.AddAddress(randIPv4Address(t), randIPv4Address(t))
	require.False(t, book.Empty())
}

func TestPrivatePeers(t *testing.T) {
	fname := createTempFileName("addrbook_test")
	defer deleteTempFile(fname)

	book := NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())

	addrs, private := testCreatePrivateAddrs(t, 10)
	book.AddPrivateIDs(private)

	// private addrs must not be added
	for _, addr := range addrs {
		err := book.AddAddress(addr, addr)
		if assert.Error(t, err) {
			_, ok := err.(ErrAddrBookPrivate)
			assert.True(t, ok)
		}
	}

	// addrs coming from private peers must not be added
	err := book.AddAddress(randIPv4Address(t), addrs[0])
	if assert.Error(t, err) {
		_, ok := err.(ErrAddrBookPrivateSrc)
		assert.True(t, ok)
	}
}

func testAddrBookAddressSelection(t *testing.T, bookSize int) {
	// generate all combinations of old (m) and new addresses
	for nBookOld := 0; nBookOld <= bookSize; nBookOld++ {
		nBookNew := bookSize - nBookOld
		dbgStr := fmt.Sprintf("book of size %d (new %d, old %d)", bookSize, nBookNew, nBookOld)

		// create book and get selection
		book, fname := createAddrBookWithMOldAndNNewAddrs(t, nBookOld, nBookNew)
		defer deleteTempFile(fname)
		addrs := book.GetSelectionWithBias(biasToSelectNewPeers)
		assert.NotNil(t, addrs, "%s - expected a non-nil selection", dbgStr)
		nAddrs := len(addrs)
		assert.NotZero(t, nAddrs, "%s - expected at least one address in selection", dbgStr)

		// check there's no nil addresses
		for _, addr := range addrs {
			if addr == nil {
				t.Fatalf("%s - got nil address in selection %v", dbgStr, addrs)
			}
		}

		// XXX: shadowing
		nOld, nNew := countOldAndNewAddrsInSelection(addrs, book)

		// Given:
		// n - num new addrs, m - num old addrs
		// k - num new addrs expected in the beginning (based on bias %)
		// i=min(n, max(k,r-m)), aka expNew
		// j=min(m, r-i), aka expOld
		//
		// We expect this layout:
		// indices:      0...i-1   i...i+j-1
		// addresses:    N0..Ni-1  O0..Oj-1
		//
		// There is at least one partition and at most three.
		var (
			k      = percentageOfNum(biasToSelectNewPeers, nAddrs)
			expNew = cmn.MinInt(nNew, cmn.MaxInt(k, nAddrs-nBookOld))
			expOld = cmn.MinInt(nOld, nAddrs-expNew)
		)

		// Verify that the number of old and new addresses are as expected
		if nNew != expNew {
			t.Fatalf("%s - expected new addrs %d, got %d", dbgStr, expNew, nNew)
		}
		if nOld != expOld {
			t.Fatalf("%s - expected old addrs %d, got %d", dbgStr, expOld, nOld)
		}

		// Verify that the order of addresses is as expected
		// Get the sequence types and lengths of the selection
		seqLens, seqTypes, err := analyseSelectionLayout(book, addrs)
		assert.NoError(t, err, "%s", dbgStr)

		// Build a list with the expected lengths of partitions and another with the expected types, e.g.:
		// expSeqLens = [10, 22], expSeqTypes = [1, 2]
		// means we expect 10 new (type 1) addresses followed by 22 old (type 2) addresses.
		var expSeqLens []int
		var expSeqTypes []int

		switch {
		case expOld == 0: // all new addresses
			expSeqLens = []int{nAddrs}
			expSeqTypes = []int{1}
		case expNew == 0: // all old addresses
			expSeqLens = []int{nAddrs}
			expSeqTypes = []int{2}
		case nAddrs-expNew-expOld == 0: // new addresses, old addresses
			expSeqLens = []int{expNew, expOld}
			expSeqTypes = []int{1, 2}
		}

		assert.Equal(t, expSeqLens, seqLens,
			"%s - expected sequence lengths of old/new %v, got %v",
			dbgStr, expSeqLens, seqLens)
		assert.Equal(t, expSeqTypes, seqTypes,
			"%s - expected sequence types (1-new, 2-old) was %v, got %v",
			dbgStr, expSeqTypes, seqTypes)
	}
}

func TestMultipleAddrBookAddressSelection(t *testing.T) {
	// test books with smaller size, < N
	const N = 32
	for bookSize := 1; bookSize < N; bookSize++ {
		testAddrBookAddressSelection(t, bookSize)
	}

	// Test for two books with sizes from following ranges
	ranges := [...][]int{{33, 100}, {100, 175}}
	bookSizes := make([]int, 0, len(ranges))
	for _, r := range ranges {
		bookSizes = append(bookSizes, cmn.RandIntn(r[1]-r[0])+r[0])
	}
	t.Logf("Testing address selection for the following book sizes %v\n", bookSizes)
	for _, bookSize := range bookSizes {
		testAddrBookAddressSelection(t, bookSize)
	}
}

func assertMOldAndNNewAddrsInSelection(t *testing.T, m, n int, addrs []*p2p.NetAddress, book *addrBook) {
	nOld, nNew := countOldAndNewAddrsInSelection(addrs, book)
	assert.Equal(t, m, nOld, "old addresses")
	assert.Equal(t, n, nNew, "new addresses")
}

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

func createAddrBookWithMOldAndNNewAddrs(t *testing.T, nOld, nNew int) (book *addrBook, fname string) {
	fname = createTempFileName("addrbook_test")

	book = NewAddrBook(fname, true)
	book.SetLogger(log.TestingLogger())
	assert.Zero(t, book.Size())

	randAddrs := randNetAddressPairs(t, nOld)
	for _, addr := range randAddrs {
		book.AddAddress(addr.addr, addr.src)
		book.MarkGood(addr.addr.ID)
	}

	randAddrs = randNetAddressPairs(t, nNew)
	for _, addr := range randAddrs {
		book.AddAddress(addr.addr, addr.src)
	}

	return
}

func countOldAndNewAddrsInSelection(addrs []*p2p.NetAddress, book *addrBook) (nOld, nNew int) {
	for _, addr := range addrs {
		if book.IsGood(addr) {
			nOld++
		} else {
			nNew++
		}
	}
	return
}

// Analyse the layout of the selection specified by 'addrs'
// Returns:
// - seqLens - the lengths of the sequences of addresses of same type
// - seqTypes - the types of sequences in selection
func analyseSelectionLayout(book *addrBook, addrs []*p2p.NetAddress) (seqLens, seqTypes []int, err error) {
	// address types are: 0 - nil, 1 - new, 2 - old
	var (
		prevType      = 0
		currentSeqLen = 0
	)

	for _, addr := range addrs {
		addrType := 0
		if book.IsGood(addr) {
			addrType = 2
		} else {
			addrType = 1
		}
		if addrType != prevType && prevType != 0 {
			seqLens = append(seqLens, currentSeqLen)
			seqTypes = append(seqTypes, prevType)
			currentSeqLen = 0
		}
		currentSeqLen++
		prevType = addrType
	}

	seqLens = append(seqLens, currentSeqLen)
	seqTypes = append(seqTypes, prevType)

	return
}
