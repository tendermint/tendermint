package account

import (
	"bytes"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
)

type Signable interface {
	WriteSignBytes(w io.Writer, n *int64, err *error)
}

func SignBytes(o Signable) []byte {
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	o.WriteSignBytes(buf, n, err)
	if *err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//-----------------------------------------------------------------------------

type Account struct {
	Address  []byte
	PubKey   PubKey
	Sequence uint
	Balance  uint64
}

func NewAccount(pubKey PubKey) *Account {
	address := pubKey.Address()
	return &Account{
		Address:  address,
		PubKey:   pubKey,
		Sequence: uint(0),
		Balance:  uint64(0),
	}
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

func AccountDecoder(r io.Reader, n *int64, err *error) interface{} {
	return ReadBinary(&Account{}, r, n, err)
}

var AccountCodec = Codec{
	Encode: AccountEncoder,
	Decode: AccountDecoder,
}
