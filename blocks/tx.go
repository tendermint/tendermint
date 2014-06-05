package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)

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
    Type()          Byte
    Binary

    Signature()     *Signature
}

const (
    TX_TYPE_SEND =      Byte(0x01)
    TX_TYPE_NAME =      Byte(0x02)
)

func ReadTx(r io.Reader) Tx {
    return nil
}


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

func (self *SendTx) Signature() *Signature {
    return self.Signature
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

func (self *NameTx) Signature() *Signature {
    return self.Signature
}
