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
