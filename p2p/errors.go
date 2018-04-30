package p2p

import (
	"errors"
	"fmt"
)

var (
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
	ErrSwitchConnectToSelf = errors.New("Connect to self")
)

type ErrSwitchAuthenticationFailure struct {
	Dialed *NetAddress
	Got    ID
}

func (e ErrSwitchAuthenticationFailure) Error() string {
	return fmt.Sprintf("Failed to authenticate peer. Dialed %v, but got peer with ID %s", e.Dialed, e.Got)
}

//-------------------------------------------------------------------

type ErrNetAddressNoID struct {
	Addr string
}

func (e ErrNetAddressNoID) Error() string {
	return fmt.Sprintf("Address (%s) does not contain ID", e.Addr)
}

type ErrNetAddressInvalid struct {
	Addr string
	Err  error
}

func (e ErrNetAddressInvalid) Error() string {
	return fmt.Sprintf("Invalid address (%s): %v", e.Addr, e.Err)
}

type ErrNetAddressLookup struct {
	Addr string
	Err  error
}

func (e ErrNetAddressLookup) Error() string {
	return fmt.Sprintf("Error looking up host (%s): %v", e.Addr, e.Err)
}
