package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "github.com/tendermint/tendermint/merkle"
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
    return &Block{
        Header:     ReadHeader(r),
        Validation: ReadValidation(r),
        Data:       ReadData(r),
    }
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

func ReadHeader(r io.Reader) Header {
    return Header{
        Name:       ReadString(r),
        Height:     ReadUInt64(r),
        Fees:       ReadUInt64(r),
        Time:       ReadUInt64(r),
        PrevHash:   ReadByteSlice(r),
        ValidationHash: ReadByteSlice(r),
        DataHash:   ReadByteSlice(r),
    }
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
    Signatures  []Signature
    Adjustments []Adjustment
}

func ReadValidation(r io.Reader) Validation {
    numSigs :=  int(ReadUInt64(r))
    numAdjs :=  int(ReadUInt64(r))
    sigs := make([]Signature,  0, numSigs)
    for i:=0; i<numSigs; i++ {
        sigs = append(sigs, ReadSignature(r))
    }
    adjs := make([]Adjustment, 0, numAdjs)
    for i:=0; i<numSigs; i++ {
        adjs = append(adjs, ReadAdjustment(r))
    }
    return Validation{
        Signatures:     sigs,
        Adjustments:    adjs,
    }
}

func (self *Validation) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = UInt64(len(self.Signatures)).WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = UInt64(len(self.Adjustments)).WriteTo(w)
    n += n_; if err != nil { return n, err }
    for _, sig := range self.Signatures {
        n_, err = sig.WriteTo(w)
        n += n_; if err != nil { return n, err }
    }
    for _, adj := range self.Adjustments {
        n_, err = adj.WriteTo(w)
        n += n_; if err != nil { return n, err }
    }
    return
}

/* Block > Data */

type Data struct {
    Txs []Tx
}

func ReadData(r io.Reader) Data {
    numTxs := int(ReadUInt64(r))
    txs := make([]Tx, 0, numTxs)
    for i:=0; i<numTxs; i++ {
        txs = append(txs, ReadTx(r))
    }
    return Data{txs}
}

func (self *Data) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = UInt64(len(self.Txs)).WriteTo(w)
    n += n_; if err != nil { return n, err }
    for _, tx := range self.Txs {
        n_, err = tx.WriteTo(w)
        n += n_; if err != nil { return }
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
