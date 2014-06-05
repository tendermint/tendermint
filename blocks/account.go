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

func ReadAccountId(r io.Reader) AccountId {
    return nil
}

/* AccountNumber < AccountId */

type AccountNumber uint64

func (self AccountNumber) Type() Byte {
    return ACCOUNT_TYPE_NUMBER
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

func (self AccountPubKey) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = ByteSlice(self).WriteTo(w)
    n += n_; return
}
