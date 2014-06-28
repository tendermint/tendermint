package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)

/*  Set

    A Set could be a bloom filter for lossy filtering, or could be a lossless filter.
*/
type Set interface {
    Binary
    Add(Msg)
    Has(Msg)    bool

    // Loads a new set.
    // Convenience factory method
    Load(ByteSlice) Set
}


/* BloomFilterSet */

type BloomFilterSet struct {
    lastBlockHeight     UInt64
    lastHeaderHeight    UInt64
}

func (bs *BloomFilterSet) WriteTo(w io.Writer) (n int64, err error) {
    n, err = WriteOnto(String("block"),     w, n, err)
    n, err = WriteOnto(bs.lastBlockHeight,  w, n, err)
    n, err = WriteOnto(bs.lastHeaderHeight, w, n, err)
    return
}

func (bs *BloomFilterSet) Add(msg Msg) {
}

func (bs *BloomFilterSet) Has(msg Msg) bool {
    return false
}

func (bs *BloomFilterSet) Load(bytes ByteSlice) Set {
    return nil
}


/* BitarraySet */

type BlockSet struct {
}

func (bs *BlockSet) WriteTo(w io.Writer) (n int64, err error) {
    return
}

func (bs *BlockSet) Add(msg Msg) {
}

func (bs *BlockSet) Has(msg Msg) bool {
    return false
}

func (bs *BlockSet) Load(bytes ByteSlice) Set {
    return nil
}
