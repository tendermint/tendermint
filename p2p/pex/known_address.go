package pex

import (
	"time"

	"github.com/tendermint/tendermint/p2p"
)

// knownAddress tracks information about a known network address
// that is used to determine how viable an address is.
type knownAddress struct {
	Addr        *p2p.NetAddress `json:"addr"`
	Src         *p2p.NetAddress `json:"src"`
	Attempts    int32           `json:"attempts"`
	LastAttempt time.Time       `json:"last_attempt"`
	LastSuccess time.Time       `json:"last_success"`
	BucketType  byte            `json:"bucket_type"`
	Buckets     []int           `json:"buckets"`
}

func newKnownAddress(addr *p2p.NetAddress, src *p2p.NetAddress) *knownAddress {
	return &knownAddress{
		Addr:        addr,
		Src:         src,
		Attempts:    0,
		LastAttempt: time.Now(),
		BucketType:  bucketTypeNew,
		Buckets:     nil,
	}
}

func (ka *knownAddress) ID() p2p.ID {
	return ka.Addr.ID
}

func (ka *knownAddress) copy() *knownAddress {
	return &knownAddress{
		Addr:        ka.Addr,
		Src:         ka.Src,
		Attempts:    ka.Attempts,
		LastAttempt: ka.LastAttempt,
		LastSuccess: ka.LastSuccess,
		BucketType:  ka.BucketType,
		Buckets:     ka.Buckets,
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
	ka.Attempts++
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
			// TODO refactor to return error?
			// log.Warn(Fmt("Bucket already exists in ka.Buckets: %v", ka))
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
		// TODO refactor to return error?
		// log.Warn(Fmt("bucketIdx not found in ka.Buckets: %v", ka))
		return -1
	}
	ka.Buckets = buckets
	return len(ka.Buckets)
}

/*
   An address is bad if the address in question is a New address, has not been tried in the last
   minute, and meets one of the following criteria:

   1) It claims to be from the future
   2) It hasn't been seen in over a week
   3) It has failed at least three times and never succeeded
   4) It has failed ten times in the last week

   All addresses that meet these criteria are assumed to be worthless and not
   worth keeping hold of.

   XXX: so a good peer needs us to call MarkGood before the conditions above are reached!
*/
func (ka *knownAddress) isBad() bool {
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
