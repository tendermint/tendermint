package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"io"
)

type AccountId struct {
	Type   Byte
	Number UInt64
	PubKey ByteSlice
}

const (
	ACCOUNT_TYPE_NUMBER = Byte(0x01)
	ACCOUNT_TYPE_PUBKEY = Byte(0x02)
	ACCOUNT_TYPE_BOTH   = Byte(0x03)
)

func ReadAccountId(r io.Reader) AccountId {
	switch t := ReadByte(r); t {
	case ACCOUNT_TYPE_NUMBER:
		return AccountId{t, ReadUInt64(r), nil}
	case ACCOUNT_TYPE_PUBKEY:
		return AccountId{t, 0, ReadByteSlice(r)}
	case ACCOUNT_TYPE_BOTH:
		return AccountId{t, ReadUInt64(r), ReadByteSlice(r)}
	default:
		Panicf("Unknown AccountId type %x", t)
		return AccountId{}
	}
}

func (self AccountId) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Type, w, n, err)
	if self.Type == ACCOUNT_TYPE_NUMBER || self.Type == ACCOUNT_TYPE_BOTH {
		n, err = WriteOnto(self.Number, w, n, err)
	}
	if self.Type == ACCOUNT_TYPE_PUBKEY || self.Type == ACCOUNT_TYPE_BOTH {
		n, err = WriteOnto(self.PubKey, w, n, err)
	}
	return
}

func AccountNumber(n UInt64) AccountId {
	return AccountId{ACCOUNT_TYPE_NUMBER, n, nil}
}
