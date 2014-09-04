package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
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
	Type() byte
	Binary
}

const (
	TX_TYPE_SEND = byte(0x01)
	TX_TYPE_NAME = byte(0x02)
)

func ReadTx(r io.Reader, n *int64, err *error) Tx {
	switch t := ReadByte(r, n, err); t {
	case TX_TYPE_SEND:
		return &SendTx{
			Fee:       ReadUInt64(r, n, err),
			To:        ReadUInt64(r, n, err),
			Amount:    ReadUInt64(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case TX_TYPE_NAME:
		return &NameTx{
			Fee:       ReadUInt64(r, n, err),
			Name:      ReadString(r, n, err),
			PubKey:    ReadByteSlice(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	default:
		Panicf("Unknown Tx type %x", t)
		return nil
	}
}

/* SendTx < Tx */

type SendTx struct {
	Fee    uint64
	To     uint64
	Amount uint64
	Signature
}

func (self *SendTx) Type() byte {
	return TX_TYPE_SEND
}

func (self *SendTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteUInt64(w, self.To, &n, &err)
	WriteUInt64(w, self.Amount, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

/* NameTx < Tx */

type NameTx struct {
	Fee    uint64
	Name   string
	PubKey []byte
	Signature
}

func (self *NameTx) Type() byte {
	return TX_TYPE_NAME
}

func (self *NameTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteString(w, self.Name, &n, &err)
	WriteByteSlice(w, self.PubKey, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}
