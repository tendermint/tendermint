package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "github.com/tendermint/tendermint/merkle"
    "io"
)

/* BlockData */

type BlockData struct {
    Txs []Tx
}

func (self *BlockData) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    for _, tx := range self.Txs {
        n_, err = tx.WriteTo(w)
        n += n_
        if err != nil { return }
    }
    return
}

func (self *BlockData) Hash() ByteSlice {
    bs := make([]Binary, 0, len(self.Txs))
    for i, tx := range self.Txs {
        bs[i] = Binary(tx)
    }
    return merkle.HashFromBinarySlice(bs)
}

func (self *BlockData) AddTx(tx Tx) {
    self.Txs = append(self.Txs, tx)
}

func NewBlockData() *BlockData {
    return &BlockData{}
}

/*

Tx wire format:

    |T|L...|MMM...|A...|SSS...|

    T  type of the tx (1 byte)
    L  length of M, varint encoded (1+ bytes)
    M  Tx bytes (L bytes)
    A  account number, varint encoded (1+ bytes)
    S  signature of all prior bytes (32 bytes)

*/

type Tx interface {
    Binary
    Type()          Byte
}

const (
    TX_TYPE_SEND =      Byte(0x00)
    TX_TYPE_BOND =      Byte(0x11)
    TX_TYPE_UNBOND =    Byte(0x12)
    TX_TYPE_NAME =      Byte(0x20)
)

/* SendTx < Tx */

type SendTx struct {
    Signature
    Fee             UInt64
    To              AccountId
    Amount          UInt64
}

func (self *SendTx) Type() Byte {
    return TX_TYPE_SEND
}

func (self *SendTx) ByteSize() int {
    return 1 +
        self.Signature.ByteSize() +
        self.Fee.ByteSize() +
        self.To.ByteSize() +
        self.Amount.ByteSize()
}

func (self *SendTx) Equals(other Binary) bool {
    if o, ok := other.(*SendTx); ok {
        return self.Signature.Equals(&o.Signature) &&
               self.Fee.Equals(o.Fee) &&
               self.To.Equals(o.To) &&
               self.Amount.Equals(o.Amount)
    } else {
        return false
    }
}

func (self *SendTx) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.To.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Amount.WriteTo(w)
    n += n_; return
}

/* BondTx < Tx */

type BondTx struct {
    Signature
    Fee             UInt64
    UnbondTo        AccountId
    Amount          UInt64
}

func (self *BondTx) Type() Byte {
    return TX_TYPE_BOND
}

func (self *BondTx) ByteSize() int {
    return 1 +
        self.Signature.ByteSize() +
        self.Fee.ByteSize() +
        self.UnbondTo.ByteSize() +
        self.Amount.ByteSize()
}

func (self *BondTx) Equals(other Binary) bool {
    if o, ok := other.(*BondTx); ok {
        return self.Signature.Equals(&o.Signature) &&
               self.Fee.Equals(o.Fee) &&
               self.UnbondTo.Equals(o.UnbondTo) &&
               self.Amount.Equals(o.Amount)
    } else {
        return false
    }
}

func (self *BondTx) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.UnbondTo.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Amount.WriteTo(w)
    n += n_; return
}

/* UnbondTx < Tx */

type UnbondTx struct {
    Signature
    Fee             UInt64
    Amount          UInt64
}

func (self *UnbondTx) Type() Byte {
    return TX_TYPE_UNBOND
}

func (self *UnbondTx) ByteSize() int {
    return 1 +
        self.Signature.ByteSize() +
        self.Fee.ByteSize() +
        self.Amount.ByteSize()
}

func (self *UnbondTx) Equals(other Binary) bool {
    if o, ok := other.(*UnbondTx); ok {
        return self.Signature.Equals(&o.Signature) &&
               self.Fee.Equals(o.Fee) &&
               self.Amount.Equals(o.Amount)
    } else {
        return false
    }
}

func (self *UnbondTx) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Amount.WriteTo(w)
    n += n_; return
}

/* NameTx < Tx */

type NameTx struct {
    Signature
    Fee             UInt64
    Name            String
    PubKey          ByteSlice
}

func (self *NameTx) Type() Byte {
    return TX_TYPE_NAME
}

func (self *NameTx) ByteSize() int {
    return 1 +
        self.Signature.ByteSize() +
        self.Fee.ByteSize() +
        self.Name.ByteSize() +
        self.PubKey.ByteSize()
}

func (self *NameTx) Equals(other Binary) bool {
    if o, ok := other.(*NameTx); ok {
        return self.Signature.Equals(&o.Signature) &&
               self.Fee.Equals(o.Fee) &&
               self.Name.Equals(o.Name) &&
               self.PubKey.Equals(o.PubKey)
    } else {
        return false
    }
}

func (self *NameTx) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Name.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.PubKey.WriteTo(w)
    n += n_; return
}
