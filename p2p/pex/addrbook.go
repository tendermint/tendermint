// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package pex

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/p2p"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	bucketTypeNew = 0x01
	bucketTypeOld = 0x02
)

// AddrBook is an address book used for tracking peers
// so we can gossip about them to others and select
// peers to dial.
// TODO: break this up?
type AddrBook interface {
	cmn.Service

	// Add our own addresses so we don't later add ourselves
	AddOurAddress(*p2p.NetAddress)
	// Check if it is our address
	OurAddress(*p2p.NetAddress) bool

	// Add and remove an address
	AddAddress(addr *p2p.NetAddress, src *p2p.NetAddress) error
	RemoveAddress(*p2p.NetAddress)

	// Check if the address is in the book
	HasAddress(*p2p.NetAddress) bool

	// Do we need more peers?
	NeedMoreAddrs() bool

	// Pick an address to dial
	PickAddress(biasTowardsNewAddrs int) *p2p.NetAddress

	// Mark address
	MarkGood(*p2p.NetAddress)
	MarkAttempt(*p2p.NetAddress)
	MarkBad(*p2p.NetAddress)

	IsGood(*p2p.NetAddress) bool

	// Send a selection of addresses to peers
	GetSelection() []*p2p.NetAddress
	// Send a selection of addresses with bias
	GetSelectionWithBias(biasTowardsNewAddrs int) []*p2p.NetAddress

	// TODO: remove
	ListOfKnownAddresses() []*knownAddress

	// Persist to disk
	Save()
}

var _ AddrBook = (*addrBook)(nil)

// addrBook - concurrency safe peer address manager.
// Implements AddrBook.
type addrBook struct {
	cmn.BaseService

	// immutable after creation
	filePath          string
	routabilityStrict bool
	key               string // random prefix for bucket placement

	// accessed concurrently
	mtx        sync.Mutex
	rand       *rand.Rand
	ourAddrs   map[string]struct{}
	addrLookup map[p2p.ID]*knownAddress // new & old
	bucketsOld []map[string]*knownAddress
	bucketsNew []map[string]*knownAddress
	nOld       int
	nNew       int

	wg sync.WaitGroup
}

// NewAddrBook creates a new address book.
// Use Start to begin processing asynchronous address updates.
func NewAddrBook(filePath string, routabilityStrict bool) *addrBook {
	am := &addrBook{
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())), // TODO: seed from outside
		ourAddrs:          make(map[string]struct{}),
		addrLookup:        make(map[p2p.ID]*knownAddress),
		filePath:          filePath,
		routabilityStrict: routabilityStrict,
	}
	am.init()
	am.BaseService = *cmn.NewBaseService(nil, "AddrBook", am)
	return am
}

// Initialize the buckets.
// When modifying this, don't forget to update loadFromFile()
func (a *addrBook) init() {
	a.key = crypto.CRandHex(24) // 24/2 * 8 = 96 bits
	// New addr buckets
	a.bucketsNew = make([]map[string]*knownAddress, newBucketCount)
	for i := range a.bucketsNew {
		a.bucketsNew[i] = make(map[string]*knownAddress)
	}
	// Old addr buckets
	a.bucketsOld = make([]map[string]*knownAddress, oldBucketCount)
	for i := range a.bucketsOld {
		a.bucketsOld[i] = make(map[string]*knownAddress)
	}
}

// OnStart implements Service.
func (a *addrBook) OnStart() error {
	if err := a.BaseService.OnStart(); err != nil {
		return err
	}
	a.loadFromFile(a.filePath)

	// wg.Add to ensure that any invocation of .Wait()
	// later on will wait for saveRoutine to terminate.
	a.wg.Add(1)
	go a.saveRoutine()

	return nil
}

// OnStop implements Service.
func (a *addrBook) OnStop() {
	a.BaseService.OnStop()
}

func (a *addrBook) Wait() {
	a.wg.Wait()
}

func (a *addrBook) FilePath() string {
	return a.filePath
}

//-------------------------------------------------------

// AddOurAddress one of our addresses.
func (a *addrBook) AddOurAddress(addr *p2p.NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.Logger.Info("Add our address to book", "addr", addr)
	a.ourAddrs[addr.String()] = struct{}{}
}

// OurAddress returns true if it is our address.
func (a *addrBook) OurAddress(addr *p2p.NetAddress) bool {
	a.mtx.Lock()
	_, ok := a.ourAddrs[addr.String()]
	a.mtx.Unlock()
	return ok
}

// AddAddress implements AddrBook - adds the given address as received from the given source.
// NOTE: addr must not be nil
func (a *addrBook) AddAddress(addr *p2p.NetAddress, src *p2p.NetAddress) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.addAddress(addr, src)
}

// RemoveAddress implements AddrBook - removes the address from the book.
func (a *addrBook) RemoveAddress(addr *p2p.NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.ID]
	if ka == nil {
		return
	}
	a.Logger.Info("Remove address from book", "addr", ka.Addr, "ID", ka.ID)
	a.removeFromAllBuckets(ka)
}

// IsGood returns true if peer was ever marked as good and haven't
// done anything wrong since then.
func (a *addrBook) IsGood(addr *p2p.NetAddress) bool {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.addrLookup[addr.ID].isOld()
}

// HasAddress returns true if the address is in the book.
func (a *addrBook) HasAddress(addr *p2p.NetAddress) bool {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.ID]
	return ka != nil
}

// NeedMoreAddrs implements AddrBook - returns true if there are not have enough addresses in the book.
func (a *addrBook) NeedMoreAddrs() bool {
	return a.Size() < needAddressThreshold
}

// PickAddress implements AddrBook. It picks an address to connect to.
// The address is picked randomly from an old or new bucket according
// to the biasTowardsNewAddrs argument, which must be between [0, 100] (or else is truncated to that range)
// and determines how biased we are to pick an address from a new bucket.
// PickAddress returns nil if the AddrBook is empty or if we try to pick
// from an empty bucket.
func (a *addrBook) PickAddress(biasTowardsNewAddrs int) *p2p.NetAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.size() == 0 {
		return nil
	}
	if biasTowardsNewAddrs > 100 {
		biasTowardsNewAddrs = 100
	}
	if biasTowardsNewAddrs < 0 {
		biasTowardsNewAddrs = 0
	}

	// Bias between new and old addresses.
	oldCorrelation := math.Sqrt(float64(a.nOld)) * (100.0 - float64(biasTowardsNewAddrs))
	newCorrelation := math.Sqrt(float64(a.nNew)) * float64(biasTowardsNewAddrs)

	// pick a random peer from a random bucket
	var bucket map[string]*knownAddress
	pickFromOldBucket := (newCorrelation+oldCorrelation)*a.rand.Float64() < oldCorrelation
	if (pickFromOldBucket && a.nOld == 0) ||
		(!pickFromOldBucket && a.nNew == 0) {
		return nil
	}
	// loop until we pick a random non-empty bucket
	for len(bucket) == 0 {
		if pickFromOldBucket {
			bucket = a.bucketsOld[a.rand.Intn(len(a.bucketsOld))]
		} else {
			bucket = a.bucketsNew[a.rand.Intn(len(a.bucketsNew))]
		}
	}
	// pick a random index and loop over the map to return that index
	randIndex := a.rand.Intn(len(bucket))
	for _, ka := range bucket {
		if randIndex == 0 {
			return ka.Addr
		}
		randIndex--
	}
	return nil
}

// MarkGood implements AddrBook - it marks the peer as good and
// moves it into an "old" bucket.
func (a *addrBook) MarkGood(addr *p2p.NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.ID]
	if ka == nil {
		return
	}
	ka.markGood()
	if ka.isNew() {
		a.moveToOld(ka)
	}
}

// MarkAttempt implements AddrBook - it marks that an attempt was made to connect to the address.
func (a *addrBook) MarkAttempt(addr *p2p.NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.ID]
	if ka == nil {
		return
	}
	ka.markAttempt()
}

// MarkBad implements AddrBook. Currently it just ejects the address.
// TODO: black list for some amount of time
func (a *addrBook) MarkBad(addr *p2p.NetAddress) {
	a.RemoveAddress(addr)
}

// GetSelection implements AddrBook.
// It randomly selects some addresses (old & new). Suitable for peer-exchange protocols.
func (a *addrBook) GetSelection() []*p2p.NetAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.size() == 0 {
		return nil
	}

	allAddr := make([]*p2p.NetAddress, a.size())
	i := 0
	for _, ka := range a.addrLookup {
		allAddr[i] = ka.Addr
		i++
	}

	numAddresses := cmn.MaxInt(
		cmn.MinInt(minGetSelection, len(allAddr)),
		len(allAddr)*getSelectionPercent/100)
	numAddresses = cmn.MinInt(maxGetSelection, numAddresses)

	// Fisher-Yates shuffle the array. We only need to do the first
	// `numAddresses' since we are throwing the rest.
	// XXX: What's the point of this if we already loop randomly through addrLookup ?
	for i := 0; i < numAddresses; i++ {
		// pick a number between current index and the end
		j := rand.Intn(len(allAddr)-i) + i
		allAddr[i], allAddr[j] = allAddr[j], allAddr[i]
	}

	// slice off the limit we are willing to share.
	return allAddr[:numAddresses]
}

// GetSelectionWithBias implements AddrBook.
// It randomly selects some addresses (old & new). Suitable for peer-exchange protocols.
//
// Each address is picked randomly from an old or new bucket according to the
// biasTowardsNewAddrs argument, which must be between [0, 100] (or else is truncated to
// that range) and determines how biased we are to pick an address from a new
// bucket.
func (a *addrBook) GetSelectionWithBias(biasTowardsNewAddrs int) []*p2p.NetAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.size() == 0 {
		return nil
	}

	if biasTowardsNewAddrs > 100 {
		biasTowardsNewAddrs = 100
	}
	if biasTowardsNewAddrs < 0 {
		biasTowardsNewAddrs = 0
	}

	numAddresses := cmn.MaxInt(
		cmn.MinInt(minGetSelection, a.size()),
		a.size()*getSelectionPercent/100)
	numAddresses = cmn.MinInt(maxGetSelection, numAddresses)

	selection := make([]*p2p.NetAddress, numAddresses)

	oldBucketToAddrsMap := make(map[int]map[string]struct{})
	var oldIndex int
	newBucketToAddrsMap := make(map[int]map[string]struct{})
	var newIndex int

	selectionIndex := 0
ADDRS_LOOP:
	for selectionIndex < numAddresses {
		pickFromOldBucket := int((float64(selectionIndex)/float64(numAddresses))*100) >= biasTowardsNewAddrs
		pickFromOldBucket = (pickFromOldBucket && a.nOld > 0) || a.nNew == 0
		bucket := make(map[string]*knownAddress)

		// loop until we pick a random non-empty bucket
		for len(bucket) == 0 {
			if pickFromOldBucket {
				oldIndex = a.rand.Intn(len(a.bucketsOld))
				bucket = a.bucketsOld[oldIndex]
			} else {
				newIndex = a.rand.Intn(len(a.bucketsNew))
				bucket = a.bucketsNew[newIndex]
			}
		}

		// pick a random index
		randIndex := a.rand.Intn(len(bucket))

		// loop over the map to return that index
		var selectedAddr *p2p.NetAddress
		for _, ka := range bucket {
			if randIndex == 0 {
				selectedAddr = ka.Addr
				break
			}
			randIndex--
		}

		// if we have selected the address before, restart the loop
		// otherwise, record it and continue
		if pickFromOldBucket {
			if addrsMap, ok := oldBucketToAddrsMap[oldIndex]; ok {
				if _, ok = addrsMap[selectedAddr.String()]; ok {
					continue ADDRS_LOOP
				}
			} else {
				oldBucketToAddrsMap[oldIndex] = make(map[string]struct{})
			}
			oldBucketToAddrsMap[oldIndex][selectedAddr.String()] = struct{}{}
		} else {
			if addrsMap, ok := newBucketToAddrsMap[newIndex]; ok {
				if _, ok = addrsMap[selectedAddr.String()]; ok {
					continue ADDRS_LOOP
				}
			} else {
				newBucketToAddrsMap[newIndex] = make(map[string]struct{})
			}
			newBucketToAddrsMap[newIndex][selectedAddr.String()] = struct{}{}
		}

		selection[selectionIndex] = selectedAddr
		selectionIndex++
	}

	return selection
}

// ListOfKnownAddresses returns the new and old addresses.
func (a *addrBook) ListOfKnownAddresses() []*knownAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	addrs := []*knownAddress{}
	for _, addr := range a.addrLookup {
		addrs = append(addrs, addr.copy())
	}
	return addrs
}

//------------------------------------------------

// Size returns the number of addresses in the book.
func (a *addrBook) Size() int {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.size()
}

func (a *addrBook) size() int {
	return a.nNew + a.nOld
}

//----------------------------------------------------------

// Save persists the address book to disk.
func (a *addrBook) Save() {
	a.saveToFile(a.filePath) // thread safe
}

func (a *addrBook) saveRoutine() {
	defer a.wg.Done()

	saveFileTicker := time.NewTicker(dumpAddressInterval)
out:
	for {
		select {
		case <-saveFileTicker.C:
			a.saveToFile(a.filePath)
		case <-a.Quit():
			break out
		}
	}
	saveFileTicker.Stop()
	a.saveToFile(a.filePath)
	a.Logger.Info("Address handler done")
}

//----------------------------------------------------------

func (a *addrBook) getBucket(bucketType byte, bucketIdx int) map[string]*knownAddress {
	switch bucketType {
	case bucketTypeNew:
		return a.bucketsNew[bucketIdx]
	case bucketTypeOld:
		return a.bucketsOld[bucketIdx]
	default:
		cmn.PanicSanity("Should not happen")
		return nil
	}
}

// Adds ka to new bucket. Returns false if it couldn't do it cuz buckets full.
// NOTE: currently it always returns true.
func (a *addrBook) addToNewBucket(ka *knownAddress, bucketIdx int) bool {
	// Sanity check
	if ka.isOld() {
		a.Logger.Error(cmn.Fmt("Cannot add address already in old bucket to a new bucket: %v", ka))
		return false
	}

	addrStr := ka.Addr.String()
	bucket := a.getBucket(bucketTypeNew, bucketIdx)

	// Already exists?
	if _, ok := bucket[addrStr]; ok {
		return true
	}

	// Enforce max addresses.
	if len(bucket) > newBucketSize {
		a.Logger.Info("new bucket is full, expiring new")
		a.expireNew(bucketIdx)
	}

	// Add to bucket.
	bucket[addrStr] = ka
	// increment nNew if the peer doesnt already exist in a bucket
	if ka.addBucketRef(bucketIdx) == 1 {
		a.nNew++
	}

	// Add it to addrLookup
	a.addrLookup[ka.ID()] = ka

	return true
}

// Adds ka to old bucket. Returns false if it couldn't do it cuz buckets full.
func (a *addrBook) addToOldBucket(ka *knownAddress, bucketIdx int) bool {
	// Sanity check
	if ka.isNew() {
		a.Logger.Error(cmn.Fmt("Cannot add new address to old bucket: %v", ka))
		return false
	}
	if len(ka.Buckets) != 0 {
		a.Logger.Error(cmn.Fmt("Cannot add already old address to another old bucket: %v", ka))
		return false
	}

	addrStr := ka.Addr.String()
	bucket := a.getBucket(bucketTypeOld, bucketIdx)

	// Already exists?
	if _, ok := bucket[addrStr]; ok {
		return true
	}

	// Enforce max addresses.
	if len(bucket) > oldBucketSize {
		return false
	}

	// Add to bucket.
	bucket[addrStr] = ka
	if ka.addBucketRef(bucketIdx) == 1 {
		a.nOld++
	}

	// Ensure in addrLookup
	a.addrLookup[ka.ID()] = ka

	return true
}

func (a *addrBook) removeFromBucket(ka *knownAddress, bucketType byte, bucketIdx int) {
	if ka.BucketType != bucketType {
		a.Logger.Error(cmn.Fmt("Bucket type mismatch: %v", ka))
		return
	}
	bucket := a.getBucket(bucketType, bucketIdx)
	delete(bucket, ka.Addr.String())
	if ka.removeBucketRef(bucketIdx) == 0 {
		if bucketType == bucketTypeNew {
			a.nNew--
		} else {
			a.nOld--
		}
		delete(a.addrLookup, ka.ID())
	}
}

func (a *addrBook) removeFromAllBuckets(ka *knownAddress) {
	for _, bucketIdx := range ka.Buckets {
		bucket := a.getBucket(ka.BucketType, bucketIdx)
		delete(bucket, ka.Addr.String())
	}
	ka.Buckets = nil
	if ka.BucketType == bucketTypeNew {
		a.nNew--
	} else {
		a.nOld--
	}
	delete(a.addrLookup, ka.ID())
}

//----------------------------------------------------------

func (a *addrBook) pickOldest(bucketType byte, bucketIdx int) *knownAddress {
	bucket := a.getBucket(bucketType, bucketIdx)
	var oldest *knownAddress
	for _, ka := range bucket {
		if oldest == nil || ka.LastAttempt.Before(oldest.LastAttempt) {
			oldest = ka
		}
	}
	return oldest
}

// adds the address to a "new" bucket. if its already in one,
// it only adds it probabilistically
func (a *addrBook) addAddress(addr, src *p2p.NetAddress) error {
	if a.routabilityStrict && !addr.Routable() {
		return fmt.Errorf("Cannot add non-routable address %v", addr)
	}
	if _, ok := a.ourAddrs[addr.String()]; ok {
		// Ignore our own listener address.
		return fmt.Errorf("Cannot add ourselves with address %v", addr)
	}

	ka := a.addrLookup[addr.ID]

	if ka != nil {
		// Already old.
		if ka.isOld() {
			return nil
		}
		// Already in max new buckets.
		if len(ka.Buckets) == maxNewBucketsPerAddress {
			return nil
		}
		// The more entries we have, the less likely we are to add more.
		factor := int32(2 * len(ka.Buckets))
		if a.rand.Int31n(factor) != 0 {
			return nil
		}
	} else {
		ka = newKnownAddress(addr, src)
	}

	bucket := a.calcNewBucket(addr, src)
	added := a.addToNewBucket(ka, bucket)
	if !added {
		a.Logger.Info("Can't add new address, addr book is full", "address", addr, "total", a.size())
	}

	a.Logger.Info("Added new address", "address", addr, "total", a.size())
	return nil
}

// Make space in the new buckets by expiring the really bad entries.
// If no bad entries are available we remove the oldest.
func (a *addrBook) expireNew(bucketIdx int) {
	for addrStr, ka := range a.bucketsNew[bucketIdx] {
		// If an entry is bad, throw it away
		if ka.isBad() {
			a.Logger.Info(cmn.Fmt("expiring bad address %v", addrStr))
			a.removeFromBucket(ka, bucketTypeNew, bucketIdx)
			return
		}
	}

	// If we haven't thrown out a bad entry, throw out the oldest entry
	oldest := a.pickOldest(bucketTypeNew, bucketIdx)
	a.removeFromBucket(oldest, bucketTypeNew, bucketIdx)
}

// Promotes an address from new to old. If the destination bucket is full,
// demote the oldest one to a "new" bucket.
// TODO: Demote more probabilistically?
func (a *addrBook) moveToOld(ka *knownAddress) {
	// Sanity check
	if ka.isOld() {
		a.Logger.Error(cmn.Fmt("Cannot promote address that is already old %v", ka))
		return
	}
	if len(ka.Buckets) == 0 {
		a.Logger.Error(cmn.Fmt("Cannot promote address that isn't in any new buckets %v", ka))
		return
	}

	// Remember one of the buckets in which ka is in.
	freedBucket := ka.Buckets[0]
	// Remove from all (new) buckets.
	a.removeFromAllBuckets(ka)
	// It's officially old now.
	ka.BucketType = bucketTypeOld

	// Try to add it to its oldBucket destination.
	oldBucketIdx := a.calcOldBucket(ka.Addr)
	added := a.addToOldBucket(ka, oldBucketIdx)
	if !added {
		// No room, must evict something
		oldest := a.pickOldest(bucketTypeOld, oldBucketIdx)
		a.removeFromBucket(oldest, bucketTypeOld, oldBucketIdx)
		// Find new bucket to put oldest in
		newBucketIdx := a.calcNewBucket(oldest.Addr, oldest.Src)
		added := a.addToNewBucket(oldest, newBucketIdx)
		// No space in newBucket either, just put it in freedBucket from above.
		if !added {
			added := a.addToNewBucket(oldest, freedBucket)
			if !added {
				a.Logger.Error(cmn.Fmt("Could not migrate oldest %v to freedBucket %v", oldest, freedBucket))
			}
		}
		// Finally, add to bucket again.
		added = a.addToOldBucket(ka, oldBucketIdx)
		if !added {
			a.Logger.Error(cmn.Fmt("Could not re-add ka %v to oldBucketIdx %v", ka, oldBucketIdx))
		}
	}
}

//---------------------------------------------------------------------
// calculate bucket placements

// doublesha256(  key + sourcegroup +
//                int64(doublesha256(key + group + sourcegroup))%bucket_per_group  ) % num_new_buckets
func (a *addrBook) calcNewBucket(addr, src *p2p.NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(a.groupKey(addr))...)
	data1 = append(data1, []byte(a.groupKey(src))...)
	hash1 := doubleSha256(data1)
	hash64 := binary.BigEndian.Uint64(hash1)
	hash64 %= newBucketsPerGroup
	var hashbuf [8]byte
	binary.BigEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, a.groupKey(src)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := doubleSha256(data2)
	return int(binary.BigEndian.Uint64(hash2) % newBucketCount)
}

// doublesha256(  key + group +
//                int64(doublesha256(key + addr))%buckets_per_group  ) % num_old_buckets
func (a *addrBook) calcOldBucket(addr *p2p.NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(addr.String())...)
	hash1 := doubleSha256(data1)
	hash64 := binary.BigEndian.Uint64(hash1)
	hash64 %= oldBucketsPerGroup
	var hashbuf [8]byte
	binary.BigEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, a.groupKey(addr)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := doubleSha256(data2)
	return int(binary.BigEndian.Uint64(hash2) % oldBucketCount)
}

// Return a string representing the network group of this address.
// This is the /16 for IPv4, the /32 (/36 for he.net) for IPv6, the string
// "local" for a local address and the string "unroutable" for an unroutable
// address.
func (a *addrBook) groupKey(na *p2p.NetAddress) string {
	if a.routabilityStrict && na.Local() {
		return "local"
	}
	if a.routabilityStrict && !na.Routable() {
		return "unroutable"
	}

	if ipv4 := na.IP.To4(); ipv4 != nil {
		return (&net.IPNet{IP: na.IP, Mask: net.CIDRMask(16, 32)}).String()
	}
	if na.RFC6145() || na.RFC6052() {
		// last four bytes are the ip address
		ip := net.IP(na.IP[12:16])
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(16, 32)}).String()
	}

	if na.RFC3964() {
		ip := net.IP(na.IP[2:7])
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(16, 32)}).String()

	}
	if na.RFC4380() {
		// teredo tunnels have the last 4 bytes as the v4 address XOR
		// 0xff.
		ip := net.IP(make([]byte, 4))
		for i, byte := range na.IP[12:16] {
			ip[i] = byte ^ 0xff
		}
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(16, 32)}).String()
	}

	// OK, so now we know ourselves to be a IPv6 address.
	// bitcoind uses /32 for everything, except for Hurricane Electric's
	// (he.net) IP range, which it uses /36 for.
	bits := 32
	heNet := &net.IPNet{IP: net.ParseIP("2001:470::"),
		Mask: net.CIDRMask(32, 128)}
	if heNet.Contains(na.IP) {
		bits = 36
	}

	return (&net.IPNet{IP: na.IP, Mask: net.CIDRMask(bits, 128)}).String()
}

// doubleSha256 calculates sha256(sha256(b)) and returns the resulting bytes.
func doubleSha256(b []byte) []byte {
	hasher := sha256.New()
	hasher.Write(b) // nolint: errcheck, gas
	sum := hasher.Sum(nil)
	hasher.Reset()
	hasher.Write(sum) // nolint: errcheck, gas
	return hasher.Sum(nil)
}
