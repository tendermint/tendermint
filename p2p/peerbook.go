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
	"os"
	"sync"
	"time"

	lpeer "github.com/libp2p/go-libp2p-peer"
	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	// peers under which the peer manager will claim to need more peers.
	needPeerThreshold = 1000

	// interval used to dump the peer cache to disk for future use.
	dumpPeerInterval = time.Minute * 2

	// max peers in each old peer bucket.
	oldBucketSize = 64

	// buckets we split old peers over.
	oldBucketCount = 64

	// max peers in each new peer bucket.
	newBucketSize = 64

	// buckets that we spread new peers over.
	newBucketCount = 256

	// old buckets over which an peer group will be spread.
	oldBucketsPerGroup = 4

	// new buckets over which a source peer group will be spread.
	newBucketsPerGroup = 32

	// buckets a frequently seen new peer may end up in.
	maxNewBucketsPerPeer = 4

	// days before which we assume an peer has vanished
	// if we have not seen it announced in that long.
	numMissingDays = 30

	// tries without a single success before we assume an peer is bad.
	numRetries = 3

	// max failures we will accept without a success before considering an peer bad.
	maxFailures = 10

	// days since the last success before we will consider evicting an peer.
	minBadDays = 7

	// % of total peers known returned by GetSelection.
	getSelectionPercent = 23

	// min peers that must be returned by GetSelection. Useful for bootstrapping.
	minGetSelection = 32

	// max peers returned by GetSelection
	// NOTE: this must match "maxPexMessageSize"
	maxGetSelection = 250
)

const (
	bucketTypeNew = 0x01
	bucketTypeOld = 0x02
)

// PeerBook - concurrency safe peer identity manager.
type PeerBook struct {
	cmn.BaseService

	// immutable after creation
	filePath string
	key      string

	// accessed concurrently
	mtx        sync.Mutex
	rand       *rand.Rand
	ourPeers   map[string]lpeer.ID
	peerLookup map[string]*knownPeer // new & old
	bucketsOld []map[string]*knownPeer
	bucketsNew []map[string]*knownPeer
	nOld       int
	nNew       int

	wg sync.WaitGroup
}

// NewPeerBook creates a new peer book.
// Use Start to begin processing asynchronous peer updates.
func NewPeerBook(filePath string) *PeerBook {
	am := &PeerBook{
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
		ourPeers:   make(map[string]lpeer.ID),
		peerLookup: make(map[string]*knownPeer),
		filePath:   filePath,
	}
	am.init()
	am.BaseService = *cmn.NewBaseService(nil, "PeerBook", am)
	return am
}

// When modifying this, don't forget to update loadFromFile()
func (a *PeerBook) init() {
	a.key = crypto.CRandHex(24) // 24/2 * 8 = 96 bits
	// New addr buckets
	a.bucketsNew = make([]map[string]*knownPeer, newBucketCount)
	for i := range a.bucketsNew {
		a.bucketsNew[i] = make(map[string]*knownPeer)
	}
	// Old addr buckets
	a.bucketsOld = make([]map[string]*knownPeer, oldBucketCount)
	for i := range a.bucketsOld {
		a.bucketsOld[i] = make(map[string]*knownPeer)
	}
}

// OnStart implements Service.
func (a *PeerBook) OnStart() error {
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
func (a *PeerBook) OnStop() {
	a.BaseService.OnStop()
}

func (a *PeerBook) Wait() {
	a.wg.Wait()
}

// AddOurPeer adds another one of our peers.
func (a *PeerBook) AddOurPeer(peer lpeer.ID) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.Logger.Info("Add our peer to book", "id", peer.Pretty())
	a.ourPeers[peer.Pretty()] = peer
}

// OurPeers returns a list of our peers.
func (a *PeerBook) OurPeers() []lpeer.ID {
	addrs := []lpeer.ID{}
	for _, addr := range a.ourPeers {
		addrs = append(addrs, addr)
	}
	return addrs
}

// AddPeer adds the given peer as received from the given source.
// NOTE: addr must not be nil
func (a *PeerBook) AddPeer(id lpeer.ID, src lpeer.ID) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.addPeer(id, src)
}

// NeedMoreAddrs returns true if there are not have enough peers in the book.
func (a *PeerBook) NeedMoreAddrs() bool {
	return a.Size() < needPeerThreshold
}

// Size returns the number of peers in the book.
func (a *PeerBook) Size() int {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.size()
}

func (a *PeerBook) size() int {
	return a.nNew + a.nOld
}

// PickPeer picks a peer to connect to.
// The peer is picked randomly from an old or new bucket according
// to the newBias argument, which must be between [0, 100] (or else is truncated to that range)
// and determines how biased we are to pick an peer from a new bucket.
// PickPeer returns nil if the PeerBook is empty or if we try to pick
// from an empty bucket.
func (a *PeerBook) PickPeer(newBias int) *string {
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

	// Bias between new and old peers.
	oldCorrelation := math.Sqrt(float64(a.nOld)) * (100.0 - float64(newBias))
	newCorrelation := math.Sqrt(float64(a.nNew)) * float64(newBias)

	// pick a random peer from a random bucket
	var bucket map[string]*knownPeer
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
			return &ka.ID
		}
		randIndex--
	}
	return nil
}

// MarkGood marks the peer as good and moves it into an "old" bucket.
// XXX: we never call this!
func (a *PeerBook) MarkGood(peer lpeer.ID) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.peerLookup[peer.Pretty()]
	if ka == nil {
		return
	}
	ka.markGood()
	if ka.isNew() {
		a.moveToOld(ka)
	}
}

// MarkAttempt marks that an attempt was made to connect to the peer.
func (a *PeerBook) MarkAttempt(peer lpeer.ID) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.peerLookup[peer.Pretty()]
	if ka == nil {
		return
	}
	ka.markAttempt()
}

// MarkBad currently just ejects the peer. In the future, consider
// blacklisting.
func (a *PeerBook) MarkBad(addr lpeer.ID) {
	a.RemovePeer(addr)
}

// RemovePeer removes the peer from the book.
func (a *PeerBook) RemovePeer(peer lpeer.ID) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ka := a.peerLookup[peer.Pretty()]
	if ka == nil {
		return
	}
	a.Logger.Info("Remove peer from book", "peer-id", peer)
	a.removeFromAllBuckets(ka)
}

/* Peer exchange */

// GetSelection randomly selects some peers (old & new). Suitable for peer-exchange protocols.
func (a *PeerBook) GetSelection() []lpeer.ID {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.size() == 0 {
		return nil
	}

	allPeers := make([]lpeer.ID, a.size())
	i := 0
	for _, v := range a.peerLookup {
		pid, _, _ := v.parseIds()
		allPeers[i] = pid
		i++
	}

	numPeeres := cmn.MaxInt(
		cmn.MinInt(minGetSelection, len(allPeers)),
		len(allPeers)*getSelectionPercent/100)
	numPeeres = cmn.MinInt(maxGetSelection, numPeeres)

	// Fisher-Yates shuffle the array. We only need to do the first
	// `numPeeres' since we are throwing the rest.
	// XXX: What's the point of this if we already loop randomly through peerLookup ?
	for i := 0; i < numPeeres; i++ {
		// pick a number between current index and the end
		j := rand.Intn(len(allPeers)-i) + i
		allPeers[i], allPeers[j] = allPeers[j], allPeers[i]
	}

	// slice off the limit we are willing to share.
	return allPeers[:numPeeres]
}

/* Loading & Saving */

type addrBookJSON struct {
	Key   string
	Addrs []*knownPeer
}

func (a *PeerBook) saveToFile(filePath string) {
	a.Logger.Info("Saving PeerBook to file", "size", a.Size())

	a.mtx.Lock()
	defer a.mtx.Unlock()
	// Compile Addrs
	addrs := []*knownPeer{}
	for _, ka := range a.peerLookup {
		addrs = append(addrs, ka)
	}

	aJSON := &addrBookJSON{
		Key:   a.key,
		Addrs: addrs,
	}

	jsonBytes, err := json.MarshalIndent(aJSON, "", "\t")
	if err != nil {
		a.Logger.Error("Failed to save PeerBook to file", "err", err)
		return
	}
	err = cmn.WriteFileAtomic(filePath, jsonBytes, 0644)
	if err != nil {
		a.Logger.Error("Failed to save PeerBook to file", "file", filePath, "err", err)
	}
}

// Returns false if file does not exist.
// cmn.Panics if file is corrupt.
func (a *PeerBook) loadFromFile(filePath string) bool {
	// If doesn't exist, do nothing.
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}

	// Load addrBookJSON{}
	r, err := os.Open(filePath)
	if err != nil {
		cmn.PanicCrisis(cmn.Fmt("Error opening file %s: %v", filePath, err))
	}
	defer r.Close() // nolint: errcheck
	aJSON := &addrBookJSON{}
	dec := json.NewDecoder(r)
	err = dec.Decode(aJSON)
	if err != nil {
		cmn.PanicCrisis(cmn.Fmt("Error reading file %s: %v", filePath, err))
	}

	// Restore all the fields...
	// Restore the key
	a.key = aJSON.Key
	// Restore .bucketsNew & .bucketsOld
	for _, ka := range aJSON.Addrs {
		for _, bucketIndex := range ka.Buckets {
			bucket := a.getBucket(ka.BucketType, bucketIndex)
			bucket[ka.ID] = ka
		}
		a.peerLookup[ka.ID] = ka
		if ka.BucketType == bucketTypeNew {
			a.nNew++
		} else {
			a.nOld++
		}
	}
	return true
}

// Save saves the book.
func (a *PeerBook) Save() {
	a.Logger.Info("Saving PeerBook to file", "size", a.Size())
	a.saveToFile(a.filePath)
}

/* Private methods */

func (a *PeerBook) saveRoutine() {
	defer a.wg.Done()

	saveFileTicker := time.NewTicker(dumpPeerInterval)
out:
	for {
		select {
		case <-saveFileTicker.C:
			a.saveToFile(a.filePath)
		case <-a.Quit:
			break out
		}
	}
	saveFileTicker.Stop()
	a.saveToFile(a.filePath)
	a.Logger.Info("Peer handler done")
}

func (a *PeerBook) getBucket(bucketType byte, bucketIdx int) map[string]*knownPeer {
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
func (a *PeerBook) addToNewBucket(ka *knownPeer, bucketIdx int) bool {
	// Sanity check
	if ka.isOld() {
		a.Logger.Error(cmn.Fmt("Cannot add peer already in old bucket to a new bucket: %v", ka))
		return false
	}

	peerIdStr := ka.ID
	bucket := a.getBucket(bucketTypeNew, bucketIdx)

	// Already exists?
	if _, ok := bucket[peerIdStr]; ok {
		return true
	}

	// Enforce max peers.
	if len(bucket) > newBucketSize {
		a.Logger.Info("new bucket is full, expiring old ")
		a.expireNew(bucketIdx)
	}

	// Add to bucket.
	bucket[peerIdStr] = ka
	if ka.addBucketRef(bucketIdx) == 1 {
		a.nNew++
	}

	// Ensure in peerLookup
	a.peerLookup[peerIdStr] = ka

	return true
}

// Adds ka to old bucket. Returns false if it couldn't do it cuz buckets full.
func (a *PeerBook) addToOldBucket(ka *knownPeer, bucketIdx int) bool {
	// Sanity check
	if ka.isNew() {
		a.Logger.Error(cmn.Fmt("Cannot add new peer to old bucket: %v", ka))
		return false
	}
	if len(ka.Buckets) != 0 {
		a.Logger.Error(cmn.Fmt("Cannot add already old peer to another old bucket: %v", ka))
		return false
	}

	peerIdStr := ka.ID
	bucket := a.getBucket(bucketTypeOld, bucketIdx)

	// Already exists?
	if _, ok := bucket[peerIdStr]; ok {
		return true
	}

	// Enforce max peers.
	if len(bucket) > oldBucketSize {
		return false
	}

	// Add to bucket.
	bucket[peerIdStr] = ka
	if ka.addBucketRef(bucketIdx) == 1 {
		a.nOld++
	}

	// Ensure in peerLookup
	a.peerLookup[peerIdStr] = ka

	return true
}

func (a *PeerBook) removeFromBucket(ka *knownPeer, bucketType byte, bucketIdx int) {
	if ka.BucketType != bucketType {
		a.Logger.Error(cmn.Fmt("Bucket type mismatch: %v", ka))
		return
	}
	bucket := a.getBucket(bucketType, bucketIdx)
	delete(bucket, ka.ID)
	if ka.removeBucketRef(bucketIdx) == 0 {
		if bucketType == bucketTypeNew {
			a.nNew--
		} else {
			a.nOld--
		}
		delete(a.peerLookup, ka.ID)
	}
}

func (a *PeerBook) removeFromAllBuckets(ka *knownPeer) {
	for _, bucketIdx := range ka.Buckets {
		bucket := a.getBucket(ka.BucketType, bucketIdx)
		delete(bucket, ka.ID)
	}
	ka.Buckets = nil
	if ka.BucketType == bucketTypeNew {
		a.nNew--
	} else {
		a.nOld--
	}
	delete(a.peerLookup, ka.ID)
}

func (a *PeerBook) pickOldest(bucketType byte, bucketIdx int) *knownPeer {
	bucket := a.getBucket(bucketType, bucketIdx)
	var oldest *knownPeer
	for _, ka := range bucket {
		if oldest == nil || ka.LastAttempt.Before(oldest.LastAttempt) {
			oldest = ka
		}
	}
	return oldest
}

func (a *PeerBook) addPeer(id lpeer.ID, src lpeer.ID) error {
	if _, ok := a.ourPeers[id.Pretty()]; ok {
		// Ignore our own listener peer.
		return fmt.Errorf("Cannot add ourselves with peer %v", id.Pretty())
	}

	ka := a.peerLookup[id.Pretty()]

	if ka != nil {
		// Already old.
		if ka.isOld() {
			return nil
		}
		// Already in max new buckets.
		if len(ka.Buckets) == maxNewBucketsPerPeer {
			return nil
		}
		// The more entries we have, the less likely we are to add more.
		factor := int32(2 * len(ka.Buckets))
		if a.rand.Int31n(factor) != 0 {
			return nil
		}
	} else {
		ka = newKnownPeer(id, src)
	}

	bucket := a.calcNewBucket(id, src)
	a.addToNewBucket(ka, bucket)

	a.Logger.Info("Added new peer", "peer", id.Pretty(), "total", a.size())
	return nil
}

// Make space in the new buckets by expiring the really bad entries.
// If no bad entries are available we remove the oldest.
func (a *PeerBook) expireNew(bucketIdx int) {
	for peerIdStr, ka := range a.bucketsNew[bucketIdx] {
		// If an entry is bad, throw it away
		if ka.isBad() {
			a.Logger.Info(cmn.Fmt("expiring bad peer %v", peerIdStr))
			a.removeFromBucket(ka, bucketTypeNew, bucketIdx)
			return
		}
	}

	// If we haven't thrown out a bad entry, throw out the oldest entry
	oldest := a.pickOldest(bucketTypeNew, bucketIdx)
	a.removeFromBucket(oldest, bucketTypeNew, bucketIdx)
}

// Promotes an peer from new to old.
// TODO: Move to old probabilistically.
// The better a node is, the less likely it should be evicted from an old bucket.
func (a *PeerBook) moveToOld(ka *knownPeer) {
	// Sanity check
	if ka.isOld() {
		a.Logger.Error(cmn.Fmt("Cannot promote peer that is already old %v", ka))
		return
	}
	if len(ka.Buckets) == 0 {
		a.Logger.Error(cmn.Fmt("Cannot promote peer that isn't in any new buckets %v", ka))
		return
	}

	// Remember one of the buckets in which ka is in.
	freedBucket := ka.Buckets[0]
	// Remove from all (new) buckets.
	a.removeFromAllBuckets(ka)
	// It's officially old now.
	ka.BucketType = bucketTypeOld

	// Try to add it to its oldBucket destination.
	kaId, _, _ := ka.parseIds()
	oldBucketIdx := a.calcOldBucket(kaId)
	added := a.addToOldBucket(ka, oldBucketIdx)
	if !added {
		// No room, must evict something
		oldest := a.pickOldest(bucketTypeOld, oldBucketIdx)
		a.removeFromBucket(oldest, bucketTypeOld, oldBucketIdx)
		// Find new bucket to put oldest in
		oldestId, oldestSrc, _ := oldest.parseIds()
		newBucketIdx := a.calcNewBucket(oldestId, oldestSrc)
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

// doublesha256(  key + sourcegroup +
//                int64(doublesha256(key + group + sourcegroup))%bucket_per_group  ) % num_new_buckets
func (a *PeerBook) calcNewBucket(addr, src lpeer.ID) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(addr)...)
	data1 = append(data1, []byte(src)...)
	hash1 := doubleSha256(data1)
	hash64 := binary.BigEndian.Uint64(hash1)
	hash64 %= newBucketsPerGroup
	var hashbuf [8]byte
	binary.BigEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, []byte(src)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := doubleSha256(data2)
	return int(binary.BigEndian.Uint64(hash2) % newBucketCount)
}

// doublesha256(  key + peerid +
//                int64(doublesha256(key + addr))%buckets_per_group  ) % num_old_buckets
func (a *PeerBook) calcOldBucket(peer lpeer.ID) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(peer.Pretty())...)
	hash1 := doubleSha256(data1)
	hash64 := binary.BigEndian.Uint64(hash1)
	hash64 %= oldBucketsPerGroup
	var hashbuf [8]byte
	binary.BigEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, []byte(peer)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := doubleSha256(data2)
	return int(binary.BigEndian.Uint64(hash2) % oldBucketCount)
}

//-----------------------------------------------------------------------------

/*
   knownPeer

   tracks information about a known network peer that is used
   to determine how viable an peer is.
*/
type knownPeer struct {
	ID          string
	Src         string
	Attempts    int32
	LastAttempt time.Time
	LastSuccess time.Time
	BucketType  byte
	Buckets     []int
}

func newKnownPeer(id lpeer.ID, src lpeer.ID) *knownPeer {
	return &knownPeer{
		ID:          id.Pretty(),
		Src:         src.Pretty(),
		Attempts:    0,
		LastAttempt: time.Now(),
		BucketType:  bucketTypeNew,
		Buckets:     nil,
	}
}

func (ka *knownPeer) parseIds() (id lpeer.ID, src lpeer.ID, err error) {
	id, err = lpeer.IDB58Decode(ka.ID)
	if err != nil {
		return
	}
	src, err = lpeer.IDB58Decode(ka.Src)
	return
}

func (ka *knownPeer) isOld() bool {
	return ka.BucketType == bucketTypeOld
}

func (ka *knownPeer) isNew() bool {
	return ka.BucketType == bucketTypeNew
}

func (ka *knownPeer) markAttempt() {
	now := time.Now()
	ka.LastAttempt = now
	ka.Attempts += 1
}

func (ka *knownPeer) markGood() {
	now := time.Now()
	ka.LastAttempt = now
	ka.Attempts = 0
	ka.LastSuccess = now
}

func (ka *knownPeer) addBucketRef(bucketIdx int) int {
	for _, bucket := range ka.Buckets {
		if bucket == bucketIdx {
			// TODO refactor to return error?
			// log.Warn(Fmt("Bucket already exists in ka.Buckets: %v", ka))
			return -1
		}
	}
	ka.Buckets = append(ka.Buckets, bucketIdx)
	return len(ka.Buckets)
}

func (ka *knownPeer) removeBucketRef(bucketIdx int) int {
	buckets := []int{}
	for _, bucket := range ka.Buckets {
		if bucket != bucketIdx {
			buckets = append(buckets, bucket)
		}
	}
	if len(buckets) != len(ka.Buckets)-1 {
		// TODO refactor to return error?
		// log.Warn(Fmt("bucketIdx not found in ka.Buckets: %v", ka))
		return -1
	}
	ka.Buckets = buckets
	return len(ka.Buckets)
}

/*
   An peer is bad if the peer in question is a New peer, has not been tried in the last
   minute, and meets one of the following criteria:

   1) It claims to be from the future
   2) It hasn't been seen in over a month
   3) It has failed at least three times and never succeeded
   4) It has failed ten times in the last week

   All peers that meet these criteria are assumed to be worthless and not
   worth keeping hold of.

   XXX: so a good peer needs us to call MarkGood before the conditions above are reached!
*/
func (ka *knownPeer) isBad() bool {
	// Is Old --> good
	if ka.BucketType == bucketTypeOld {
		return false
	}

	// Has been attempted in the last minute --> good
	if ka.LastAttempt.Before(time.Now().Add(-1 * time.Minute)) {
		return false
	}

	// Too old?
	// XXX: does this mean if we've kept a connection up for this long we'll disconnect?!
	// and shouldn't it be .Before ?
	if ka.LastAttempt.After(time.Now().Add(-1 * numMissingDays * time.Hour * 24)) {
		return true
	}

	// Never succeeded?
	if ka.LastSuccess.IsZero() && ka.Attempts >= numRetries {
		return true
	}

	// Hasn't succeeded in too long?
	// XXX: does this mean if we've kept a connection up for this long we'll disconnect?!
	if ka.LastSuccess.Before(time.Now().Add(-1*minBadDays*time.Hour*24)) &&
		ka.Attempts >= maxFailures {
		return true
	}

	return false
}
