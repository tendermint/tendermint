package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)

/*
The full vote structure is only needed when presented as evidence.
Typically only the signature is passed around, as the hash & height are implied.
*/

type Vote struct {
    Height      UInt64
    BlockHash   ByteSlice
    Signature
}

func ReadVote(r io.Reader) Vote {
    return Vote{
        Height:     ReadUInt64(r),
        BlockHash:  ReadByteSlice(r),
        Signature:  ReadSignature(r),
    }
}

func (self *Vote) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Height.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.BlockHash.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; return
}
