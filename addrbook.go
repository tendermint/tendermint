// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package p2p

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	crypto "github.com/tendermint/go-crypto"
)

const (
	// addresses under which the address manager will claim to need more addresses.
	needAddressThreshold = 1000

	// interval used to dump the address cache to disk for future use.
	dumpAddressInterval = time.Minute * 2

	// max addresses in each old address bucket.
	oldBucketSize = 64

	// buckets we split old addresses over.
	oldBucketCount = 64

	// max addresses in each new address bucket.
	newBucketSize = 64

	// buckets that we spread new addresses over.
	newBucketCount = 256

	// old buckets over which an address group will be spread.
	oldBucketsPerGroup = 4

	// new buckets over which an source address group will be spread.
	newBucketsPerGroup = 32

	// buckets a frequently seen new address may end up in.
	maxNewBucketsPerAddress = 4

	// days before which we assume an address has vanished
	// if we have not seen it announced in that long.
	numMissingDays = 30

	// tries without a single success before we assume an address is bad.
	numRetries = 3

	// max failures we will accept without a success before considering an address bad.
	maxFailures = 10

	// days since the last success before we will consider evicting an address.
	minBadDays = 7

	// % of total addresses known returned by GetSelection.
	getSelectionPercent = 23

	// min addresses that must be returned by GetSelection. Useful for bootstrapping.
	minGetSelection = 32

	// max addresses returned by GetSelection
	// NOTE: this must match "maxPexMessageSize"
	maxGetSelection = 250

	// current version of the on-disk format.
	serializationVersion = 1
)

const (
	bucketTypeNew = 0x01
	bucketTypeOld = 0x02
)

// AddrBook - concurrency safe peer address manager.
type AddrBook struct {
	BaseService

	mtx               sync.Mutex
	filePath          string
	routabilityStrict bool
	rand              *rand.Rand
	key               string
	ourAddrs          map[string]*NetAddress
	addrLookup        map[string]*knownAddress // new & old
	addrNew           []map[string]*knownAddress
	addrOld           []map[string]*knownAddress
	wg                sync.WaitGroup
	nOld              int
	nNew              int
}

// NewAddrBook creates a new address book.
// Use Start to begin processing asynchronous address updates.
func NewAddrBook(filePath string, routabilityStrict bool) *AddrBook {
	am := &AddrBook{
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
		ourAddrs:          make(map[string]*NetAddress),
		addrLookup:        make(map[string]*knownAddress),
		filePath:          filePath,
		routabilityStrict: routabilityStrict,
	}
	am.init()
	am.BaseService = *NewBaseService(log, "AddrBook", am)
	return am
}

// When modifying this, don't forget to update loadFromFile()
func (a *AddrBook) init() {
	a.key = crypto.CRandHex(24) // 24/2 * 8 = 96 bits
	// New addr buckets
	a.addrNew = make([]map[string]*knownAddress, newBucketCount)
	for i := range a.addrNew {
		a.addrNew[i] = make(map[string]*knownAddress)
	}
	// Old addr buckets
	a.addrOld = make([]map[string]*knownAddress, oldBucketCount)
	for i := range a.addrOld {
		a.addrOld[i] = make(map[string]*knownAddress)
	}
}

// OnStart implements Service.
func (a *AddrBook) OnStart() error {
	a.BaseService.OnStart()
	a.loadFromFile(a.filePath)
	a.wg.Add(1)
	go a.saveRoutine()
	return nil
}

// OnStop implements Service.
func (a *AddrBook) OnStop() {
	a.BaseService.OnStop()
}

func (a *AddrBook) Wait() {
	a.wg.Wait()
}

func (a *AddrBook) AddOurAddress(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	log.Info("Add our address to book", "addr", addr)
	a.ourAddrs[addr.String()] = addr
}

func (a *AddrBook) OurAddresses() []*NetAddress {
	addrs := []*NetAddress{}
	for _, addr := range a.ourAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

// NOTE: addr must not be nil
func (a *AddrBook) AddAddress(addr *NetAddress, src *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	log.Info("Add address to book", "addr", addr, "src", src)
	a.addAddress(addr, src)
}

func (a *AddrBook) NeedMoreAddrs() bool {
	return a.Size() < needAddressThreshold
}

func (a *AddrBook) Size() int {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.size()
}

func (a *AddrBook) size() int {
	return a.nNew + a.nOld
}

// Pick an address to connect to with new/old bias.
func (a *AddrBook) PickAddress(newBias int) *NetAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.size() == 0 {
		return nil
	}
	if newBias > 100 {
		newBias = 100
	}
	if newBias < 0 {
		newBias = 0
	}

	// Bias between new and old addresses.
	oldCorrelation := math.Sqrt(float64(a.nOld)) * (100.0 - float64(newBias))
	newCorrelation := math.Sqrt(float64(a.nNew)) * float64(newBias)

	if (newCorrelation+oldCorrelation)*a.rand.Float64() < oldCorrelation {
		// pick random Old bucket.
		var bucket map[string]*knownAddress = nil
		for len(bucket) == 0 {
			bucket = a.addrOld[a.rand.Intn(len(a.addrOld))]
		}
		// pick a random ka from bucket.
		randIndex := a.rand.Intn(len(bucket))
		for _, ka := range bucket {
			if randIndex == 0 {
				return ka.Addr
			}
			randIndex--
		}
		PanicSanity("Should not happen")
	} else {
		// pick random New bucket.
		var bucket map[string]*knownAddress = nil
		for len(bucket) == 0 {
			bucket = a.addrNew[a.rand.Intn(len(a.addrNew))]
		}
		// pick a random ka from bucket.
		randIndex := a.rand.Intn(len(bucket))
		for _, ka := range bucket {
			if randIndex == 0 {
				return ka.Addr
			}
			randIndex--
		}
		PanicSanity("Should not happen")
	}
	return nil
}

func (a *AddrBook) MarkGood(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.String()]
	if ka == nil {
		return
	}
	ka.markGood()
	if ka.isNew() {
		a.moveToOld(ka)
	}
}

func (a *AddrBook) MarkAttempt(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.String()]
	if ka == nil {
		return
	}
	ka.markAttempt()
}

// MarkBad currently just ejects the address. In the future, consider
// blacklisting.
func (a *AddrBook) MarkBad(addr *NetAddress) {
	a.RemoveAddress(addr)
}

// RemoveAddress removes the address from the book.
func (a *AddrBook) RemoveAddress(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.String()]
	if ka == nil {
		return
	}
	log.Info("Remove address from book", "addr", addr)
	a.removeFromAllBuckets(ka)
}

/* Peer exchange */

// GetSelection randomly selects some addresses (old & new). Suitable for peer-exchange protocols.
func (a *AddrBook) GetSelection() []*NetAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.size() == 0 {
		return nil
	}

	allAddr := make([]*NetAddress, a.size())
	i := 0
	for _, v := range a.addrLookup {
		allAddr[i] = v.Addr
		i++
	}

	numAddresses := MaxInt(
		MinInt(minGetSelection, len(allAddr)),
		len(allAddr)*getSelectionPercent/100)
	numAddresses = MinInt(maxGetSelection, numAddresses)

	// Fisher-Yates shuffle the array. We only need to do the first
	// `numAddresses' since we are throwing the rest.
	for i := 0; i < numAddresses; i++ {
		// pick a number between current index and the end
		j := rand.Intn(len(allAddr)-i) + i
		allAddr[i], allAddr[j] = allAddr[j], allAddr[i]
	}

	// slice off the limit we are willing to share.
	return allAddr[:numAddresses]
}

/* Loading & Saving */

type addrBookJSON struct {
	Key   string
	Addrs []*knownAddress
}

func (a *AddrBook) saveToFile(filePath string) {
	log.Info("Saving AddrBook to file", "size", a.Size())

	a.mtx.Lock()
	defer a.mtx.Unlock()
	// Compile Addrs
	addrs := []*knownAddress{}
	for _, ka := range a.addrLookup {
		addrs = append(addrs, ka)
	}

	aJSON := &addrBookJSON{
		Key:   a.key,
		Addrs: addrs,
	}

	jsonBytes, err := json.MarshalIndent(aJSON, "", "\t")
	if err != nil {
		log.Error("Failed to save AddrBook to file", "err", err)
		return
	}
	err = WriteFileAtomic(filePath, jsonBytes, 0644)
	if err != nil {
		log.Error("Failed to save AddrBook to file", "file", filePath, "error", err)
	}
}

// Returns false if file does not exist.
// Panics if file is corrupt.
func (a *AddrBook) loadFromFile(filePath string) bool {
	// If doesn't exist, do nothing.
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}

	// Load addrBookJSON{}
	r, err := os.Open(filePath)
	if err != nil {
		PanicCrisis(Fmt("Error opening file %s: %v", filePath, err))
	}
	defer r.Close()
	aJSON := &addrBookJSON{}
	dec := json.NewDecoder(r)
	err = dec.Decode(aJSON)
	if err != nil {
		PanicCrisis(Fmt("Error reading file %s: %v", filePath, err))
	}

	// Restore all the fields...
	// Restore the key
	a.key = aJSON.Key
	// Restore .addrNew & .addrOld
	for _, ka := range aJSON.Addrs {
		for _, bucketIndex := range ka.Buckets {
			bucket := a.getBucket(ka.BucketType, bucketIndex)
			bucket[ka.Addr.String()] = ka
		}
		a.addrLookup[ka.Addr.String()] = ka
		if ka.BucketType == bucketTypeNew {
			a.nNew++
		} else {
			a.nOld++
		}
	}
	return true
}

// Save saves the book.
func (a *AddrBook) Save() {
	log.Info("Saving AddrBook to file", "size", a.Size())
	a.saveToFile(a.filePath)
}

/* Private methods */

func (a *AddrBook) saveRoutine() {
	dumpAddressTicker := time.NewTicker(dumpAddressInterval)
out:
	for {
		select {
		case <-dumpAddressTicker.C:
			a.saveToFile(a.filePath)
		case <-a.Quit:
			break out
		}
	}
	dumpAddressTicker.Stop()
	a.saveToFile(a.filePath)
	a.wg.Done()
	log.Notice("Address handler done")
}

func (a *AddrBook) getBucket(bucketType byte, bucketIdx int) map[string]*knownAddress {
	switch bucketType {
	case bucketTypeNew:
		return a.addrNew[bucketIdx]
	case bucketTypeOld:
		return a.addrOld[bucketIdx]
	default:
		PanicSanity("Should not happen")
		return nil
	}
}

// Adds ka to new bucket. Returns false if it couldn't do it cuz buckets full.
// NOTE: currently it always returns true.
func (a *AddrBook) addToNewBucket(ka *knownAddress, bucketIdx int) bool {
	// Sanity check
	if ka.isOld() {
		log.Warn(Fmt("Cannot add address already in old bucket to a new bucket: %v", ka))
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
		log.Notice("new bucket is full, expiring old ")
		a.expireNew(bucketIdx)
	}

	// Add to bucket.
	bucket[addrStr] = ka
	if ka.addBucketRef(bucketIdx) == 1 {
		a.nNew++
	}

	// Ensure in addrLookup
	a.addrLookup[addrStr] = ka

	return true
}

// Adds ka to old bucket. Returns false if it couldn't do it cuz buckets full.
func (a *AddrBook) addToOldBucket(ka *knownAddress, bucketIdx int) bool {
	// Sanity check
	if ka.isNew() {
		log.Warn(Fmt("Cannot add new address to old bucket: %v", ka))
		return false
	}
	if len(ka.Buckets) != 0 {
		log.Warn(Fmt("Cannot add already old address to another old bucket: %v", ka))
		return false
	}

	addrStr := ka.Addr.String()
	bucket := a.getBucket(bucketTypeNew, bucketIdx)

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
	a.addrLookup[addrStr] = ka

	return true
}

func (a *AddrBook) removeFromBucket(ka *knownAddress, bucketType byte, bucketIdx int) {
	if ka.BucketType != bucketType {
		log.Warn(Fmt("Bucket type mismatch: %v", ka))
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
		delete(a.addrLookup, ka.Addr.String())
	}
}

func (a *AddrBook) removeFromAllBuckets(ka *knownAddress) {
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
	delete(a.addrLookup, ka.Addr.String())
}

func (a *AddrBook) pickOldest(bucketType byte, bucketIdx int) *knownAddress {
	bucket := a.getBucket(bucketType, bucketIdx)
	var oldest *knownAddress
	for _, ka := range bucket {
		if oldest == nil || ka.LastAttempt.Before(oldest.LastAttempt) {
			oldest = ka
		}
	}
	return oldest
}

func (a *AddrBook) addAddress(addr, src *NetAddress) {
	if a.routabilityStrict && !addr.Routable() {
		log.Warn(Fmt("Cannot add non-routable address %v", addr))
		return
	}
	if _, ok := a.ourAddrs[addr.String()]; ok {
		// Ignore our own listener address.
		return
	}

	ka := a.addrLookup[addr.String()]

	if ka != nil {
		// Already old.
		if ka.isOld() {
			return
		}
		// Already in max new buckets.
		if len(ka.Buckets) == maxNewBucketsPerAddress {
			return
		}
		// The more entries we have, the less likely we are to add more.
		factor := int32(2 * len(ka.Buckets))
		if a.rand.Int31n(factor) != 0 {
			return
		}
	} else {
		ka = newKnownAddress(addr, src)
	}

	bucket := a.calcNewBucket(addr, src)
	a.addToNewBucket(ka, bucket)

	log.Notice("Added new address", "address", addr, "total", a.size())
}

// Make space in the new buckets by expiring the really bad entries.
// If no bad entries are available we remove the oldest.
func (a *AddrBook) expireNew(bucketIdx int) {
	for addrStr, ka := range a.addrNew[bucketIdx] {
		// If an entry is bad, throw it away
		if ka.isBad() {
			log.Notice(Fmt("expiring bad address %v", addrStr))
			a.removeFromBucket(ka, bucketTypeNew, bucketIdx)
			return
		}
	}

	// If we haven't thrown out a bad entry, throw out the oldest entry
	oldest := a.pickOldest(bucketTypeNew, bucketIdx)
	a.removeFromBucket(oldest, bucketTypeNew, bucketIdx)
}

// Promotes an address from new to old.
// TODO: Move to old probabilistically.
// The better a node is, the less likely it should be evicted from an old bucket.
func (a *AddrBook) moveToOld(ka *knownAddress) {
	// Sanity check
	if ka.isOld() {
		log.Warn(Fmt("Cannot promote address that is already old %v", ka))
		return
	}
	if len(ka.Buckets) == 0 {
		log.Warn(Fmt("Cannot promote address that isn't in any new buckets %v", ka))
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
				log.Warn(Fmt("Could not migrate oldest %v to freedBucket %v", oldest, freedBucket))
			}
		}
		// Finally, add to bucket again.
		added = a.addToOldBucket(ka, oldBucketIdx)
		if !added {
			log.Warn(Fmt("Could not re-add ka %v to oldBucketIdx %v", ka, oldBucketIdx))
		}
	}
}

// doublesha256(  key + sourcegroup +
//                int64(doublesha256(key + group + sourcegroup))%bucket_per_group  ) % num_new_buckets
func (a *AddrBook) calcNewBucket(addr, src *NetAddress) int {
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
func (a *AddrBook) calcOldBucket(addr *NetAddress) int {
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
// This is the /16 for IPv6, the /32 (/36 for he.net) for IPv6, the string
// "local" for a local address and the string "unroutable for an unroutable
// address.
func (a *AddrBook) groupKey(na *NetAddress) string {
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

//-----------------------------------------------------------------------------

/*
   knownAddress

   tracks information about a known network address that is used
   to determine how viable an address is.
*/
type knownAddress struct {
	Addr        *NetAddress
	Src         *NetAddress
	Attempts    int32
	LastAttempt time.Time
	LastSuccess time.Time
	BucketType  byte
	Buckets     []int
}

func newKnownAddress(addr *NetAddress, src *NetAddress) *knownAddress {
	return &knownAddress{
		Addr:        addr,
		Src:         src,
		Attempts:    0,
		LastAttempt: time.Now(),
		BucketType:  bucketTypeNew,
		Buckets:     nil,
	}
}

func (ka *knownAddress) isOld() bool {
	return ka.BucketType == bucketTypeOld
}

func (ka *knownAddress) isNew() bool {
	return ka.BucketType == bucketTypeNew
}

func (ka *knownAddress) markAttempt() {
	now := time.Now()
	ka.LastAttempt = now
	ka.Attempts += 1
}

func (ka *knownAddress) markGood() {
	now := time.Now()
	ka.LastAttempt = now
	ka.Attempts = 0
	ka.LastSuccess = now
}

func (ka *knownAddress) addBucketRef(bucketIdx int) int {
	for _, bucket := range ka.Buckets {
		if bucket == bucketIdx {
			log.Warn(Fmt("Bucket already exists in ka.Buckets: %v", ka))
			return -1
		}
	}
	ka.Buckets = append(ka.Buckets, bucketIdx)
	return len(ka.Buckets)
}

func (ka *knownAddress) removeBucketRef(bucketIdx int) int {
	buckets := []int{}
	for _, bucket := range ka.Buckets {
		if bucket != bucketIdx {
			buckets = append(buckets, bucket)
		}
	}
	if len(buckets) != len(ka.Buckets)-1 {
		log.Warn(Fmt("bucketIdx not found in ka.Buckets: %v", ka))
		return -1
	}
	ka.Buckets = buckets
	return len(ka.Buckets)
}

/*
   An address is bad if the address in question has not been tried in the last
   minute and meets one of the following criteria:

   1) It claims to be from the future
   2) It hasn't been seen in over a month
   3) It has failed at least three times and never succeeded
   4) It has failed ten times in the last week

   All addresses that meet these criteria are assumed to be worthless and not
   worth keeping hold of.
*/
func (ka *knownAddress) isBad() bool {
	// Has been attempted in the last minute --> good
	if ka.LastAttempt.Before(time.Now().Add(-1 * time.Minute)) {
		return false
	}

	// Over a month old?
	if ka.LastAttempt.After(time.Now().Add(-1 * numMissingDays * time.Hour * 24)) {
		return true
	}

	// Never succeeded?
	if ka.LastSuccess.IsZero() && ka.Attempts >= numRetries {
		return true
	}

	// Hasn't succeeded in too long?
	if ka.LastSuccess.Before(time.Now().Add(-1*minBadDays*time.Hour*24)) &&
		ka.Attempts >= maxFailures {
		return true
	}

	return false
}
