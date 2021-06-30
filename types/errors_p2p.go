package types

import (
	"fmt"
)

//-------------------------------------------------------------------

type ErrNetAddressNoID struct {
	Addr string
}

func (e ErrNetAddressNoID) Error() string {
	return fmt.Sprintf("address (%s) does not contain ID", e.Addr)
}

type ErrNetAddressInvalid struct {
	Addr string
	Err  error
}

func (e ErrNetAddressInvalid) Error() string {
	return fmt.Sprintf("invalid address (%s): %v", e.Addr, e.Err)
}

type ErrNetAddressLookup struct {
	Addr string
	Err  error
}

func (e ErrNetAddressLookup) Error() string {
	return fmt.Sprintf("error looking up host (%s): %v", e.Addr, e.Err)
}
