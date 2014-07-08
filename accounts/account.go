package accounts

import (
	. "github.com/tendermint/tendermint/binary"
)

type Account struct {
	Name   String
	PubKey ByteSlice
}

func (self *Account) Verify(msg ByteSlice, sig ByteSlice) bool {
	return false
}

type MyAccount struct {
	Account
	PrivKey ByteSlice
}

func (self *MyAccount) Sign(msg ByteSlice) ByteSlice {
	return nil
}
