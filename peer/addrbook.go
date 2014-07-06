// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package peer

import (
	crand "crypto/rand" // for seeding
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
)

/* AddrBook - concurrency safe peer address manager */
type AddrBook struct {
	filePath string

	mtx       sync.Mutex
	rand      *rand.Rand
	key       [32]byte
	addrIndex map[string]*knownAddress // new & old
	addrNew   [newBucketCount]map[string]*knownAddress
	addrOld   [oldBucketCount][]*knownAddress
	started   int32
	shutdown  int32
	wg        sync.WaitGroup
	quit      chan struct{}
	nOld      int
	nNew      int
}

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
	newBucketsPerAddress = 4

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
	a.addrIndex = make(map[string]*knownAddress)
	io.ReadFull(crand.Reader, a.key[:])
	for i := range a.addrNew {
		a.addrNew[i] = make(map[string]*knownAddress)
	}
	for i := range a.addrOld {
		a.addrOld[i] = make([]*knownAddress, 0, oldBucketSize)
	}
}

func (a *AddrBook) Start() {
	if atomic.AddInt32(&a.started, 1) != 1 {
		return
	}
	log.Trace("Starting address manager")
	a.loadFromFile(a.filePath)
	a.wg.Add(1)
	go a.addressHandler()
}

func (a *AddrBook) Stop() {
	if atomic.AddInt32(&a.shutdown, 1) != 1 {
		return
	}
	log.Infof("Address manager shutting down")
	close(a.quit)
	a.wg.Wait()
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
func (a *AddrBook) PickAddress(newBias int) *knownAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.nOld == 0 && a.nNew == 0 {
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
		var bucket []*knownAddress = nil
		for len(bucket) == 0 {
			bucket = a.addrOld[a.rand.Intn(len(a.addrOld))]
		}
		// pick a random ka from bucket.
		return bucket[a.rand.Intn(len(bucket))]
	} else {
		// pick random New bucket.
		var bucket map[string]*knownAddress = nil
		for len(bucket) == 0 {
			bucket = a.addrNew[a.rand.Intn(len(a.addrNew))]
		}
		// pick a random ka from bucket.
		randIndex := a.rand.Intn(len(bucket))
		for _, ka := range bucket {
			randIndex--
			if randIndex == 0 {
				return ka
			}
		}
		panic("Should not happen")
	}
	return nil
}

func (a *AddrBook) MarkGood(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrIndex[addr.String()]
	if ka == nil {
		return
	}
	ka.MarkGood()
	if ka.OldBucket == -1 {
		a.moveToOld(ka)
	}
}

func (a *AddrBook) MarkAttempt(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.addrIndex[addr.String()]
	if ka == nil {
		return
	}
	ka.MarkAttempt()
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
	for _, v := range a.addrIndex {
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
	Key     [32]byte
	AddrNew [newBucketCount][]*knownAddress
	AddrOld [oldBucketCount][]*knownAddress
	NumOld  int
	NumNew  int
}

func (a *AddrBook) saveToFile(filePath string) {
	// turn a.addrNew into an array like a.addrOld
	__addrNew := [newBucketCount][]*knownAddress{}
	for i, newBucket := range a.addrNew {
		var array []*knownAddress = make([]*knownAddress, 0)
		for _, ka := range newBucket {
			array = append(array, ka)
		}
		__addrNew[i] = array
	}

	aJSON := &addrBookJSON{
		Key:     a.key,
		AddrNew: __addrNew,
		AddrOld: a.addrOld,
		NumOld:  a.nOld,
		NumNew:  a.nNew,
	}

	w, err := os.Create(filePath)
	if err != nil {
		log.Error("Error opening file: ", filePath, err)
		return
	}
	enc := json.NewEncoder(w)
	defer w.Close()
	err = enc.Encode(&aJSON)
	if err != nil {
		panic(err)
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
		panic(fmt.Errorf("%s error opening file: %v", filePath, err))
	}
	defer r.Close()

	aJSON := &addrBookJSON{}
	dec := json.NewDecoder(r)
	err = dec.Decode(aJSON)
	if err != nil {
		panic(fmt.Errorf("error reading %s: %v", filePath, err))
	}

	// Now we need to restore the fields

	// Restore the key
	copy(a.key[:], aJSON.Key[:])
	// Restore .addrNew
	for i, newBucket := range aJSON.AddrNew {
		for _, ka := range newBucket {
			a.addrNew[i][ka.Addr.String()] = ka
			a.addrIndex[ka.Addr.String()] = ka
		}
	}
	// Restore .addrOld
	for i, oldBucket := range aJSON.AddrOld {
		copy(a.addrOld[i], oldBucket)
		for _, ka := range oldBucket {
			a.addrIndex[ka.Addr.String()] = ka
		}
	}
	// Restore simple fields
	a.nNew = aJSON.NumNew
	a.nOld = aJSON.NumOld
}

/* Private methods */

func (a *AddrBook) addressHandler() {
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
	log.Trace("Address handler done")
}

func (a *AddrBook) addAddress(addr, src *NetAddress) {
	if !addr.Routable() {
		return
	}

	key := addr.String()
	ka := a.addrIndex[key]

	if ka != nil {
		// Already added
		if ka.OldBucket != -1 {
			return
		}
		if ka.NewRefs == newBucketsPerAddress {
			return
		}

		// The more entries we have, the less likely we are to add more.
		factor := int32(2 * ka.NewRefs)
		if a.rand.Int31n(factor) != 0 {
			return
		}
	} else {
		ka = NewknownAddress(addr, src)
		a.addrIndex[key] = ka
		a.nNew++
	}

	bucket := a.getNewBucket(addr, src)

	// Already exists?
	if _, ok := a.addrNew[bucket][key]; ok {
		return
	}

	// Enforce max addresses.
	if len(a.addrNew[bucket]) > newBucketSize {
		log.Tracef("new bucket is full, expiring old ")
		a.expireNew(bucket)
	}

	// Add to new bucket.
	ka.NewRefs++
	a.addrNew[bucket][key] = ka

	log.Tracef("Added new address %s for a total of %d addresses", addr, a.nOld+a.nNew)
}

// Make space in the new buckets by expiring the really bad entries.
// If no bad entries are available we look at a few and remove the oldest.
func (a *AddrBook) expireNew(bucket int) {
	var oldest *knownAddress
	for k, v := range a.addrNew[bucket] {
		// If an entry is bad, throw it away
		if v.IsBad() {
			log.Tracef("expiring bad address %v", k)
			delete(a.addrNew[bucket], k)
			v.NewRefs--
			if v.NewRefs == 0 {
				a.nNew--
				delete(a.addrIndex, k)
			}
			return
		}
		// or, keep track of the oldest entry
		if oldest == nil {
			oldest = v
		} else if v.LastAttempt.Before(oldest.LastAttempt.Time) {
			oldest = v
		}
	}

	// If we haven't thrown out a bad entry, throw out the oldest entry
	if oldest != nil {
		key := oldest.Addr.String()
		log.Tracef("expiring oldest address %v", key)
		delete(a.addrNew[bucket], key)
		oldest.NewRefs--
		if oldest.NewRefs == 0 {
			a.nNew--
			delete(a.addrIndex, key)
		}
	}
}

func (a *AddrBook) moveToOld(ka *knownAddress) {
	// Remove from all new buckets.
	// Remember one of those new buckets.
	addrKey := ka.Addr.String()
	freedBucket := -1
	for i := range a.addrNew {
		// we check for existance so we can record the first one
		if _, ok := a.addrNew[i][addrKey]; ok {
			delete(a.addrNew[i], addrKey)
			ka.NewRefs--
			if freedBucket == -1 {
				freedBucket = i
			}
		}
	}
	a.nNew--
	if freedBucket == -1 {
		panic("Expected to find addr in at least one new bucket")
	}

	oldBucket := a.getOldBucket(ka.Addr)

	// If room in oldBucket, put it in.
	if len(a.addrOld[oldBucket]) < oldBucketSize {
		ka.OldBucket = Int16(oldBucket)
		a.addrOld[oldBucket] = append(a.addrOld[oldBucket], ka)
		a.nOld++
		return
	}

	// No room, we have to evict something else.
	rmkaIndex := a.pickOld(oldBucket)
	rmka := a.addrOld[oldBucket][rmkaIndex]

	// Find a new bucket to put rmka in.
	newBucket := a.getNewBucket(rmka.Addr, rmka.Src)
	if len(a.addrNew[newBucket]) >= newBucketSize {
		newBucket = freedBucket
	}

	// Replace with ka in list.
	ka.OldBucket = Int16(oldBucket)
	a.addrOld[oldBucket][rmkaIndex] = ka
	rmka.OldBucket = -1

	// Put rmka into new bucket
	rmkey := rmka.Addr.String()
	log.Tracef("Replacing %s with %s in old", rmkey, addrKey)
	a.addrNew[newBucket][rmkey] = rmka
	rmka.NewRefs++
	a.nNew++
}

// Returns the index in old bucket of oldest entry.
func (a *AddrBook) pickOld(bucket int) int {
	var oldest *knownAddress
	var oldestIndex int
	for i, ka := range a.addrOld[bucket] {
		if oldest == nil || ka.LastAttempt.Before(oldest.LastAttempt.Time) {
			oldest = ka
			oldestIndex = i
		}
	}
	return oldestIndex
}

// doublesha256(key + sourcegroup +
//              int64(doublesha256(key + group + sourcegroup))%bucket_per_source_group) % num_new_buckes
func (a *AddrBook) getNewBucket(addr, src *NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, a.key[:]...)
	data1 = append(data1, []byte(GroupKey(addr))...)
	data1 = append(data1, []byte(GroupKey(src))...)
	hash1 := DoubleSha256(data1)
	hash64 := binary.LittleEndian.Uint64(hash1)
	hash64 %= newBucketsPerGroup
	var hashbuf [8]byte
	binary.LittleEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, a.key[:]...)
	data2 = append(data2, GroupKey(src)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := DoubleSha256(data2)
	return int(binary.LittleEndian.Uint64(hash2) % newBucketCount)
}

// doublesha256(key + group + truncate_to_64bits(doublesha256(key + addr))%buckets_per_group) % num_buckets
func (a *AddrBook) getOldBucket(addr *NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, a.key[:]...)
	data1 = append(data1, []byte(addr.String())...)
	hash1 := DoubleSha256(data1)
	hash64 := binary.LittleEndian.Uint64(hash1)
	hash64 %= oldBucketsPerGroup
	var hashbuf [8]byte
	binary.LittleEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, a.key[:]...)
	data2 = append(data2, GroupKey(addr)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := DoubleSha256(data2)
	return int(binary.LittleEndian.Uint64(hash2) % oldBucketCount)
}

// Return a string representing the network group of this address.
// This is the /16 for IPv6, the /32 (/36 for he.net) for IPv6, the string
// "local" for a local address and the string "unroutable for an unroutable
// address.
func GroupKey(na *NetAddress) string {
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

/*
   knownAddress

   tracks information about a known network address that is used
   to determine how viable an address is.
*/
type knownAddress struct {
	Addr        *NetAddress
	Src         *NetAddress
	Attempts    UInt32
	LastAttempt Time
	LastSuccess Time
	NewRefs     UInt16
	OldBucket   Int16
}

func NewknownAddress(addr *NetAddress, src *NetAddress) *knownAddress {
	return &knownAddress{
		Addr:        addr,
		Src:         src,
		OldBucket:   -1,
		LastAttempt: Time{time.Now()},
		Attempts:    0,
	}
}

func ReadknownAddress(r io.Reader) *knownAddress {
	return &knownAddress{
		Addr:        ReadNetAddress(r),
		Src:         ReadNetAddress(r),
		Attempts:    ReadUInt32(r),
		LastAttempt: ReadTime(r),
		LastSuccess: ReadTime(r),
		NewRefs:     ReadUInt16(r),
		OldBucket:   ReadInt16(r),
	}
}

func (ka *knownAddress) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(ka.Addr, w, n, err)
	n, err = WriteOnto(ka.Src, w, n, err)
	n, err = WriteOnto(ka.Attempts, w, n, err)
	n, err = WriteOnto(ka.LastAttempt, w, n, err)
	n, err = WriteOnto(ka.LastSuccess, w, n, err)
	n, err = WriteOnto(ka.NewRefs, w, n, err)
	n, err = WriteOnto(ka.OldBucket, w, n, err)
	return
}

func (ka *knownAddress) MarkAttempt() {
	now := Time{time.Now()}
	ka.LastAttempt = now
	ka.Attempts += 1
}

func (ka *knownAddress) MarkGood() {
	now := Time{time.Now()}
	ka.LastAttempt = now
	ka.Attempts = 0
	ka.LastSuccess = now
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
func (ka *knownAddress) IsBad() bool {
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
