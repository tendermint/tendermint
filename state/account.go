package state

import (
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
)

type Account struct {
	Address  []byte
	PubKey   PubKey
	Sequence uint
	Balance  uint64
}

func NewAccount(address []byte, pubKey PubKey) *Account {
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
