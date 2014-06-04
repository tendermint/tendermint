package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)

type AccountId interface {
    Binary
    Type()          Byte
}

const (
    ACCOUNT_TYPE_NUMBER =   Byte(0x00)
    ACCOUNT_TYPE_PUBKEY =   Byte(0x01)
)

/* AccountNumber < AccountId */

type AccountNumber uint64

func (self AccountNumber) Type() Byte {
    return ACCOUNT_TYPE_NUMBER
}

func (self AccountNumber) Equals(o Binary) bool {
    return self == o
}

func (self AccountNumber) ByteSize() int {
    return 1 + 8
}

func (self AccountNumber) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = UInt64(self).WriteTo(w)
    n += n_; return
}

/* AccountPubKey < AccountId */

type AccountPubKey []byte

func (self AccountPubKey) Type() Byte {
    return ACCOUNT_TYPE_PUBKEY
}

func (self AccountPubKey) Equals(o Binary) bool {
    return ByteSlice(self).Equals(o)
}

func (self AccountPubKey) ByteSize() int {
    return 1 + ByteSlice(self).ByteSize()
}

func (self AccountPubKey) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = ByteSlice(self).WriteTo(w)
    n += n_; return
}

/*

Signature message wire format:

    |A...|SSS...|

    A  account number, varint encoded (1+ bytes)
    S  signature of all prior bytes (32 bytes)

It usually follows the message to be signed.

*/

type Signature struct {
    Signer          AccountId
    SigBytes        ByteSlice
}

func (self *Signature) Equals(other Binary) bool {
    if o, ok := other.(*Signature); ok {
        return self.Signer.Equals(o.Signer) &&
               self.SigBytes.Equals(o.SigBytes)
    } else {
        return false
    }
}

func (self *Signature) ByteSize() int {
    return self.Signer.ByteSize() +
           self.SigBytes.ByteSize()
}

func (self *Signature) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Signer.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.SigBytes.WriteTo(w)
    n += n_; return
}

func (self *Signature) Verify(msg ByteSlice) bool {
    return false
}

func ReadSignature(buf []byte, start int) (*Signature, int) {
    return nil, 0
}

