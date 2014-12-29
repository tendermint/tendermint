package account

import (
	"bytes"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
)

// Signable is an interface for all signable things.
// It typically removes signatures before serializing.
type Signable interface {
	WriteSignBytes(w io.Writer, n *int64, err *error)
}

// SignBytes is a convenience method for getting the bytes to sign of a Signable.
func SignBytes(o Signable) []byte {
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	o.WriteSignBytes(buf, n, err)
	if *err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//-----------------------------------------------------------------------------

// Account resides in the application state, and is mutated by transactions
// on the blockchain.
// Serialized by binary.[read|write]Reflect
type Account struct {
	Address  []byte
	PubKey   PubKey
	Sequence uint
	Balance  uint64
}

func (account *Account) Copy() *Account {
	accountCopy := *account
	return &accountCopy
}

func (account *Account) String() string {
	return fmt.Sprintf("Account{%X:%v}", account.Address, account.PubKey)
}

func AccountEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	WriteBinary(o.(*Account), w, n, err)
}

func AccountDecoder(r Unreader, n *int64, err *error) interface{} {
	return ReadBinary(&Account{}, r, n, err)
}

var AccountCodec = Codec{
	Encode: AccountEncoder,
	Decode: AccountDecoder,
}
