// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package peer

import (
    . "github.com/tendermint/tendermint/binary"
    crand "crypto/rand" // for seeding
    "encoding/binary"
    "io"
    "math"
    "math/rand"
    "net"
    "sync"
    "sync/atomic"
    "time"
)

/* AddrManager - concurrency safe peer address manager */
type AddrManager struct {
    rand            *rand.Rand
    key             [32]byte
    addrIndex       map[string]*KnownAddress // addr.String() -> KnownAddress
    addrNew         [newBucketCount]map[string]*KnownAddress
    addrOld         [oldBucketCount][]*KnownAddress
    started         int32
    shutdown        int32
    wg              sync.WaitGroup
    quit            chan bool
    nOld            int
    nNew            int
    localAddresses  map[string]*localAddress
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

    // max addresses that we will send in response to a getAddr
    // (in practise the most addresses we will return from a call to AddressCache()).
    getAddrMax = 2500

    // % of total addresses known that we will share with a call to AddressCache.
    getAddrPercent = 23

    // current version of the on-disk format.
    serialisationVersion = 1
)

// Use Start to begin processing asynchronous address updates.
func NewAddrManager() *AddrManager {
    am := AddrManager{
        rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
        quit:           make(chan bool),
        localAddresses: make(map[string]*localAddress),
    }
    am.init()
    return &am
}

func (a *AddrManager) init() {
    a.addrIndex = make(map[string]*KnownAddress)
    io.ReadFull(crand.Reader, a.key[:])
    for i := range a.addrNew {
        a.addrNew[i] = make(map[string]*KnownAddress)
    }
    for i := range a.addrOld {
        a.addrOld[i] = make([]*KnownAddress, 0, oldBucketSize)
    }
}

func (a *AddrManager) Start() {
    if atomic.AddInt32(&a.started, 1) != 1 { return }
    amgrLog.Trace("Starting address manager")
    a.wg.Add(1)
    a.loadPeers()
    go a.addressHandler()
}

func (a *AddrManager) Stop() {
    if atomic.AddInt32(&a.shutdown, 1) != 1 { return }
    amgrLog.Infof("Address manager shutting down")
    close(a.quit)
    a.wg.Wait()
}

func (a *AddrManager) AddAddress(addr *NetAddress, src *NetAddress) {
    // XXX use a channel for concurrency
    a.addAddress(addr, src)
}

func (a *AddrManager) NeedMoreAddresses() bool {
    return a.NumAddresses() < needAddressThreshold
}

func (a *AddrManager) NumAddresses() int {
    return a.nOld + a.nNew
}

// Pick a new address to connect to.
func (a *AddrManager) PickAddress(class string, newBias int) *KnownAddress {
    if a.NumAddresses() == 0 { return nil }
    if newBias > 100 { newBias = 100 }
    if newBias < 0 { newBias = 0 }

    // Bias between new and old addresses.
    oldCorrelation := math.Sqrt(float64(a.nOld)) * (100.0 - float64(newBias))
    newCorrelation := math.Sqrt(float64(a.nNew)) * float64(newBias)

    if (newCorrelation+oldCorrelation)*a.rand.Float64() < oldCorrelation {
        // Old entry.
        // XXX
    } else {
        // New entry.
        // XXX
    }
    return nil
}

func (a *AddrManager) MarkGood(ka *KnownAddress) {
    ka.MarkAttempt(true)
    a.moveToOld(ka)
}

/* Loading & Saving */

func (a *AddrManager) loadPeers() {
}

func (a *AddrManager) savePeers() {
}

/* Private methods */

func (a *AddrManager) addressHandler() {
    dumpAddressTicker := time.NewTicker(dumpAddressInterval)
out:
    for {
        select {
        case <-dumpAddressTicker.C:
            a.savePeers()
        case <-a.quit:
            break out
        }
    }
    dumpAddressTicker.Stop()
    a.savePeers()
    a.wg.Done()
    amgrLog.Trace("Address handler done")
}

func (a *AddrManager) addAddress(addr, src *NetAddress) {
    if !addr.Routable() { return }

    key := addr.String()
    ka := a.addrIndex[key]

    if ka != nil {
        // Already added
        if ka.OldBucket != -1 { return }
        if ka.NewRefs == newBucketsPerAddress { return }

        // The more entries we have, the less likely we are to add more.
        factor := int32(2 * ka.NewRefs)
        if a.rand.Int31n(factor) != 0 {
            return
        }
    } else {
        ka = NewKnownAddress(addr, src)
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
        amgrLog.Tracef("new bucket is full, expiring old ")
        a.expireNew(bucket)
    }

    // Add to new bucket.
    ka.NewRefs++
    a.addrNew[bucket][key] = ka

    amgrLog.Tracef("Added new address %s for a total of %d addresses", addr, a.nOld+a.nNew)
}

// Make space in the new buckets by expiring the really bad entries.
// If no bad entries are available we look at a few and remove the oldest.
func (a *AddrManager) expireNew(bucket int) {
    var oldest *KnownAddress
    for k, v := range a.addrNew[bucket] {
        // If an entry is bad, throw it away
        if v.Bad() {
            amgrLog.Tracef("expiring bad address %v", k)
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
        } else if v.LastAttempt < oldest.LastAttempt {
            oldest = v
        }
    }

    // If we haven't thrown out a bad entry, throw out the oldest entry
    if oldest != nil {
        key := oldest.Addr.String()
        amgrLog.Tracef("expiring oldest address %v", key)
        delete(a.addrNew[bucket], key)
        oldest.NewRefs--
        if oldest.NewRefs == 0 {
            a.nNew--
            delete(a.addrIndex, key)
        }
    }
}

func (a *AddrManager) moveToOld(ka *KnownAddress) {
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
    if freedBucket == -1 { panic("Expected to find addr in at least one new bucket") }

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

    // replace with ka in list.
    ka.OldBucket = Int16(oldBucket)
    a.addrOld[oldBucket][rmkaIndex] = ka
    rmka.OldBucket = -1

    // put rmka into new bucket
    rmkey := rmka.Addr.String()
    amgrLog.Tracef("Replacing %s with %s in old", rmkey, addrKey)
    a.addrNew[newBucket][rmkey] = rmka
    rmka.NewRefs++
    a.nNew++
}

// Returns the index in old bucket of oldest entry.
func (a *AddrManager) pickOld(bucket int) int {
    var oldest *KnownAddress
    var oldestIndex int
    for i, ka := range a.addrOld[bucket] {
        if oldest == nil || ka.LastAttempt < oldest.LastAttempt {
            oldest = ka
            oldestIndex = i
        }
    }
    return oldestIndex
}

// doublesha256(key + sourcegroup +
//              int64(doublesha256(key + group + sourcegroup))%bucket_per_source_group) % num_new_buckes
func (a *AddrManager) getNewBucket(addr, src *NetAddress) int {
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
func (a *AddrManager) getOldBucket(addr *NetAddress) int {
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


///// LOCAL ADDRESS

// addressPrio is an enum type used to describe the heirarchy of local address
// discovery methods.
type addressPrio int

const (
    InterfacePrio addressPrio = iota // address of local interface.
    BoundPrio                        // Address explicitly bound to.
    UpnpPrio                         // External IP discovered from UPnP
    HttpPrio                         // Obtained from internet service.
    ManualPrio                       // provided by --externalip.
)

type localAddress struct {
    Addr    *NetAddress
    Score   addressPrio
}

// addLocalAddress adds addr to the list of known local addresses to advertise
// with the given priority.
func (a *AddrManager) addLocalAddress(addr *NetAddress, priority addressPrio) {
    // sanity check.
    if !addr.Routable() {
        amgrLog.Debugf("rejecting address %s:%d due to routability", addr.IP, addr.Port)
        return
    }
    amgrLog.Debugf("adding address %s:%d", addr.IP, addr.Port)

    key := addr.String()
    la, ok := a.localAddresses[key]
    if !ok || la.Score < priority {
        if ok {
            la.Score = priority + 1
        } else {
            a.localAddresses[key] = &localAddress{
                Addr:   addr,
                Score:  priority,
            }
        }
    }
}

// getBestLocalAddress returns the most appropriate local address that we know
// of to be contacted by rna (remote net address)
func (a *AddrManager) getBestLocalAddress(rna *NetAddress) *NetAddress {
    bestReach := 0
    var bestScore addressPrio
    var bestAddr *NetAddress
    for _, la := range a.localAddresses {
        reach := rna.ReachabilityTo(la.Addr)
        if reach > bestReach ||
            (reach == bestReach && la.Score > bestScore) {
            bestReach = reach
            bestScore = la.Score
            bestAddr = la.Addr
        }
    }
    if bestAddr != nil {
        amgrLog.Debugf("Suggesting address %s:%d for %s:%d",
            bestAddr.IP, bestAddr.Port, rna.IP, rna.Port)
    } else {
        amgrLog.Debugf("No worthy address for %s:%d",
            rna.IP, rna.Port)
        // Send something unroutable if nothing suitable.
        bestAddr = &NetAddress{
            IP:        net.IP([]byte{0, 0, 0, 0}),
            Port:      0,
        }
    }

    return bestAddr
}


// Return a string representing the network group of this address.
// This is the /16 for IPv6, the /32 (/36 for he.net) for IPv6, the string
// "local" for a local address and the string "unroutable for an unroutable
// address.
func GroupKey (na *NetAddress) string {
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
