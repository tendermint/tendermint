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
}

const (
    TX_TYPE_SEND =      Byte(0x01)
    TX_TYPE_NAME =      Byte(0x02)
)

func ReadTx(r io.Reader) Tx {
    switch t := ReadByte(r); t {
    case TX_TYPE_SEND:
        return &SendTx{
            Fee:        ReadUInt64(r),
            To:         ReadAccountId(r),
            Amount:     ReadUInt64(r),
            Signature:  ReadSignature(r),
        }
    case TX_TYPE_NAME:
        return &NameTx{
            Fee:        ReadUInt64(r),
            Name:       ReadString(r),
            PubKey:     ReadByteSlice(r),
            Signature:  ReadSignature(r),
        }
    default:
        panicf("Unknown Tx type %x", t)
        return nil
    }
}


/* SendTx < Tx */

type SendTx struct {
    Fee             UInt64
    To              AccountId
    Amount          UInt64
    Signature
}

func (self *SendTx) Type() Byte {
    return TX_TYPE_SEND
}

func (self *SendTx) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.To.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Amount.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; return
}


/* NameTx < Tx */

type NameTx struct {
    Fee             UInt64
    Name            String
    PubKey          ByteSlice
    Signature
}

func (self *NameTx) Type() Byte {
    return TX_TYPE_NAME
}

func (self *NameTx) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Name.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.PubKey.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; return
}
