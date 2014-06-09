package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "time"
    "io"
)

/*
    KnownAddress

    tracks information about a known network address that is used
    to determine how viable an address is.
*/
type KnownAddress struct {
    Addr            *NetAddress
    Src             *NetAddress
    Attempts        UInt32
    LastAttempt     UInt64
    LastSuccess     UInt64
    NewRefs         UInt16
    OldBucket       Int16  // TODO init to -1
}

func NewKnownAddress(addr *NetAddress, src *NetAddress) *KnownAddress {
    return &KnownAddress{
        Addr:           addr,
        Src:            src,
        OldBucket:      -1,
        LastAttempt:    UInt64(time.Now().Unix()),
        Attempts:       0,
    }
}

func ReadKnownAddress(r io.Reader) *KnownAddress {
    return &KnownAddress{
        Addr:           ReadNetAddress(r),
        Src:            ReadNetAddress(r),
        Attempts:       ReadUInt32(r),
        LastAttempt:    ReadUInt64(r),
        LastSuccess:    ReadUInt64(r),
        NewRefs:        ReadUInt16(r),
        OldBucket:      ReadInt16(r),
    }
}

func (ka *KnownAddress) WriteTo(w io.Writer) (n int64, err error) {
    n, err = WriteOnto(ka.Addr,             w, n, err)
    n, err = WriteOnto(ka.Src,              w, n, err)
    n, err = WriteOnto(ka.Attempts,         w, n, err)
    n, err = WriteOnto(ka.LastAttempt,      w, n, err)
    n, err = WriteOnto(ka.LastSuccess,      w, n, err)
    n, err = WriteOnto(ka.NewRefs,          w, n, err)
    n, err = WriteOnto(ka.OldBucket,        w, n, err)
    return
}

func (ka *KnownAddress) MarkAttempt(success bool) {
    now := UInt64(time.Now().Unix())
    ka.LastAttempt = now
    if success {
        ka.LastSuccess = now
        ka.Attempts = 0
    } else {
        ka.Attempts += 1
    }
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
func (ka *KnownAddress) Bad() bool {
    // Has been attempted in the last minute --> good
    if ka.LastAttempt < UInt64(time.Now().Add(-1 * time.Minute).Unix()) {
        return false
    }

    // Over a month old?
    if ka.LastAttempt > UInt64(time.Now().Add(-1 * numMissingDays * time.Hour * 24).Unix()) {
        return true
    }

    // Never succeeded?
    if ka.LastSuccess == 0 && ka.Attempts >= numRetries {
        return true
    }

    // Hasn't succeeded in too long?
    if ka.LastSuccess < UInt64(time.Now().Add(-1*minBadDays*time.Hour*24).Unix()) &&
        ka.Attempts >= maxFailures {
        return true
    }

    return false
}
