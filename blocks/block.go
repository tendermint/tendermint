package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)


/* Block */

type Block struct {
    Header
    Validation
    Data
    // Checkpoint
}

func ReadBlock(r io.Reader) *Block {
    return nil
}

func (self *Block) Validate() bool {
    return false
}


/* Block > Header */

type Header struct {
    Name            String
    Height          UInt64
    Fees            UInt64
    Time            UInt64
    PrevHash        ByteSlice
    ValidationHash  ByteSlice
    DataHash        ByteSlice
}

func ReadHeader(r io.Reader) *Header {
    return nil
}

func (self *Header) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Name.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Height.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fees.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Time.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.PrevHash.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.ValidationHash.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.DataHash.WriteTo(w)
    n += n_; return
}


/* Block > Validation */

type Validation struct {
    Signatures  []*Signature
    Adjustments []Adjustment
}

func ReadValidation(r io.Reader) *Validation {
    return nil
}


/* Block > Data */

type Data struct {
    Txs []Tx
}

func ReadData(r io.Reader) *Data {
    return nil
}

func (self *Data) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    for _, tx := range self.Txs {
        n_, err = tx.WriteTo(w)
        n += n_
        if err != nil { return }
    }
    return
}

func (self *Data) MerkleHash() ByteSlice {
    bs := make([]Binary, 0, len(self.Txs))
    for i, tx := range self.Txs {
        bs[i] = Binary(tx)
    }
    return merkle.HashFromBinarySlice(bs)
}
