// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package p2p

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/common"
)

/* AddrBook - concurrency safe peer address manager */
type AddrBook struct {
	filePath string

	mtx        sync.Mutex
	rand       *rand.Rand
	key        string
	addrLookup map[string]*knownAddress // new & old
	addrNew    []map[string]*knownAddress
	addrOld    []map[string]*knownAddress
	started    uint32
	stopped    uint32
	wg         sync.WaitGroup
	quit       chan struct{}
	nOld       int
	nNew       int
}

const (
	bucketTypeNew = 0x01
	bucketTypeOld = 0x02
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

	// max addresses that we will send in response to a GetSelection
	getSelectionMax = 2500

	// % of total addresses known that we will share with a call to GetSelection
	getSelectionPercent = 23

	// current version of the on-disk format.
	serializationVersion = 1
)

// Use Start to begin processing asynchronous address updates.
func NewAddrBook(filePath string) *AddrBook {
	am := AddrBook{
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		quit:     make(chan struct{}),
		filePath: filePath,
	}
	am.init()
	return &am
}

// When modifying this, don't forget to update loadFromFile()
func (a *AddrBook) init() {
	a.key = RandHex(24) // 24/2 * 8 = 96 bits
	// addr -> ka index
	a.addrLookup = make(map[string]*knownAddress)
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

func (a *AddrBook) Start() {
	if atomic.CompareAndSwapUint32(&a.started, 0, 1) {
		log.Info("Starting address manager")
		a.loadFromFile(a.filePath)
		a.wg.Add(1)
		go a.saveHandler()
	}
}

func (a *AddrBook) Stop() {
	if atomic.CompareAndSwapUint32(&a.stopped, 0, 1) {
		log.Info("Stopping address manager")
		close(a.quit)
		a.wg.Wait()
	}
}

func (a *AddrBook) AddAddress(addr *NetAddress, src *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.addAddress(addr, src)
}

func (a *AddrBook) NeedMoreAddresses() bool {
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
		panic("Should not happen")
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
		panic("Should not happen")
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

func (a *AddrBook) MarkBad(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrLookup[addr.String()]
	if ka == nil {
		return
	}
	// We currently just eject the address.
	// In the future, consider blacklisting.
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

	numAddresses := len(allAddr) * getSelectionPercent / 100
	if numAddresses > getSelectionMax {
		numAddresses = getSelectionMax
	}

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
	// Compile Addrs
	addrs := []*knownAddress{}
	for _, ka := range a.addrLookup {
		addrs = append(addrs, ka)
	}

	aJSON := &addrBookJSON{
		Key:   a.key,
		Addrs: addrs,
	}

	w, err := os.Create(filePath)
	if err != nil {
		log.Error("Error opening file: ", filePath, err)
		return
	}
	defer w.Close()
	jsonBytes, err := json.MarshalIndent(aJSON, "", "\t")
	_, err = w.Write(jsonBytes)
	if err != nil {
		log.Error("Failed to save AddrBook to file %v: %v", filePath, err)
	}
}

func (a *AddrBook) loadFromFile(filePath string) {
	// If doesn't exist, do nothing.
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return
	}

	// Load addrBookJSON{}
	r, err := os.Open(filePath)
	if err != nil {
		panic(fmt.Errorf("Error opening file %s: %v", filePath, err))
	}
	defer r.Close()
	aJSON := &addrBookJSON{}
	dec := json.NewDecoder(r)
	err = dec.Decode(aJSON)
	if err != nil {
		panic(fmt.Errorf("Error reading file %s: %v", filePath, err))
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
}

/* Private methods */

func (a *AddrBook) saveHandler() {
	dumpAddressTicker := time.NewTicker(dumpAddressInterval)
out:
	for {
		select {
		case <-dumpAddressTicker.C:
			a.saveToFile(a.filePath)
		case <-a.quit:
			break out
		}
	}
	dumpAddressTicker.Stop()
	a.saveToFile(a.filePath)
	a.wg.Done()
	log.Info("Address handler done")
}

func (a *AddrBook) getBucket(bucketType byte, bucketIdx int) map[string]*knownAddress {
	switch bucketType {
	case bucketTypeNew:
		return a.addrNew[bucketIdx]
	case bucketTypeOld:
		return a.addrOld[bucketIdx]
	default:
		panic("Should not happen")
	}
}

// Adds ka to new bucket. Returns false if it couldn't do it cuz buckets full.
// NOTE: currently it always returns true.
func (a *AddrBook) addToNewBucket(ka *knownAddress, bucketIdx int) bool {
	// Sanity check
	if ka.isOld() {
		log.Warning("Cannot add address already in old bucket to a new bucket: %v", ka)
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
		log.Info("new bucket is full, expiring old ")
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
		log.Warning("Cannot add new address to old bucket: %v", ka)
		return false
	}
	if len(ka.Buckets) != 0 {
		log.Warning("Cannot add already old address to another old bucket: %v", ka)
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
		log.Warning("Bucket type mismatch: %v", ka)
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
	if !addr.Routable() {
		log.Warning("Cannot add non-routable address %v", addr)
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

	log.Info("Added new address %s for a total of %d addresses", addr, a.size())
}

// Make space in the new buckets by expiring the really bad entries.
// If no bad entries are available we remove the oldest.
func (a *AddrBook) expireNew(bucketIdx int) {
	for addrStr, ka := range a.addrNew[bucketIdx] {
		// If an entry is bad, throw it away
		if ka.isBad() {
			log.Info("expiring bad address %v", addrStr)
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
		log.Warning("Cannot promote address that is already old %v", ka)
		return
	}
	if len(ka.Buckets) == 0 {
		log.Warning("Cannot promote address that isn't in any new buckets %v", ka)
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
				log.Warning("Could not migrate oldest %v to freedBucket %v", oldest, freedBucket)
			}
		}
		// Finally, add to bucket again.
		added = a.addToOldBucket(ka, oldBucketIdx)
		if !added {
			log.Warning("Could not re-add ka %v to oldBucketIdx %v", ka, oldBucketIdx)
		}
	}
}

// doublesha256(key + sourcegroup +
//              int64(doublesha256(key + group + sourcegroup))%bucket_per_source_group) % num_new_buckes
func (a *AddrBook) calcNewBucket(addr, src *NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(groupKey(addr))...)
	data1 = append(data1, []byte(groupKey(src))...)
	hash1 := doubleSha256(data1)
	hash64 := binary.LittleEndian.Uint64(hash1)
	hash64 %= newBucketsPerGroup
	var hashbuf [8]byte
	binary.LittleEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, groupKey(src)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := doubleSha256(data2)
	return int(binary.LittleEndian.Uint64(hash2) % newBucketCount)
}

// doublesha256(key + group + truncate_to_64bits(doublesha256(key + addr))%buckets_per_group) % num_buckets
func (a *AddrBook) calcOldBucket(addr *NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(addr.String())...)
	hash1 := doubleSha256(data1)
	hash64 := binary.LittleEndian.Uint64(hash1)
	hash64 %= oldBucketsPerGroup
	var hashbuf [8]byte
	binary.LittleEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, groupKey(addr)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := doubleSha256(data2)
	return int(binary.LittleEndian.Uint64(hash2) % oldBucketCount)
}

// Return a string representing the network group of this address.
// This is the /16 for IPv6, the /32 (/36 for he.net) for IPv6, the string
// "local" for a local address and the string "unroutable for an unroutable
// address.
func groupKey(na *NetAddress) string {
	if na.Local() {
		return "local"
	}
	if !na.Routable() {
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
	Attempts    uint32
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
			log.Warning("Bucket already exists in ka.Buckets: %v", ka)
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
		log.Warning("bucketIdx not found in ka.Buckets: %v", ka)
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
