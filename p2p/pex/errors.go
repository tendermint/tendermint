package pex

import (
	"fmt"

	"github.com/tendermint/tendermint/p2p"
)

type ErrAddrBookNonRoutable struct {
	Addr *p2p.NetAddress
}

func (err ErrAddrBookNonRoutable) Error() string {
	return fmt.Sprintf("Cannot add non-routable address %v", err.Addr)
}

type ErrAddrBookSelf struct {
	Addr *p2p.NetAddress
}

func (err ErrAddrBookSelf) Error() string {
	return fmt.Sprintf("Cannot add ourselves with address %v", err.Addr)
}

type ErrAddrBookPrivate struct {
	Addr *p2p.NetAddress
}

func (err ErrAddrBookPrivate) Error() string {
	return fmt.Sprintf("Cannot add private peer with address %v", err.Addr)
}

type ErrAddrBookPrivateSrc struct {
	Src *p2p.NetAddress
}

func (err ErrAddrBookPrivateSrc) Error() string {
	return fmt.Sprintf("Cannot add peer coming from private peer with address %v", err.Src)
}

type ErrAddrBookNilAddr struct {
	Addr *p2p.NetAddress
	Src  *p2p.NetAddress
}

func (err ErrAddrBookNilAddr) Error() string {
	return fmt.Sprintf("Cannot add a nil address. Got (addr, src) = (%v, %v)", err.Addr, err.Src)
}

type ErrAddrBookInvalidAddr struct {
	Addr *p2p.NetAddress
}

func (err ErrAddrBookInvalidAddr) Error() string {
	return fmt.Sprintf("Cannot add invalid address %v", err.Addr)
}

type ErrAddrBookInvalidAddrNoID struct {
	Addr *p2p.NetAddress
}

func (err ErrAddrBookInvalidAddrNoID) Error() string {
	return fmt.Sprintf("Cannot add address with no ID %v", err.Addr)
}
