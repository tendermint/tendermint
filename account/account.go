package account

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/binary"
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
	Address     []byte
	PubKey      PubKey
	Sequence    uint
	Balance     uint64
	Code        []byte // VM code
	StorageRoot []byte // VM storage merkle root.
}

func (acc *Account) Copy() *Account {
	accCopy := *acc
	return &accCopy
}

func (acc *Account) String() string {
	return fmt.Sprintf("Account{%X:%v C:%v S:%X}", acc.Address, acc.PubKey, len(acc.Code), acc.StorageRoot)
}

func AccountEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	binary.WriteBinary(o.(*Account), w, n, err)
}

func AccountDecoder(r io.Reader, n *int64, err *error) interface{} {
	return binary.ReadBinary(&Account{}, r, n, err)
}

var AccountCodec = binary.Codec{
	Encode: AccountEncoder,
	Decode: AccountDecoder,
}
