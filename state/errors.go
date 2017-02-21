package state

import (
	. "github.com/tendermint/go-common"
)

type (
	ErrInvalidBlock error
	ErrProxyAppConn error

	ErrUnknownBlock struct {
		Height int
	}

	ErrBlockHashMismatch struct {
		CoreHash []byte
		AppHash  []byte
		Height   int
	}

	ErrAppBlockHeightTooHigh struct {
		CoreHeight int
		AppHeight  int
	}

	ErrLastStateMismatch struct {
		Height int
		Core   []byte
		App    []byte
	}

	ErrStateMismatch struct {
		Got      *State
		Expected *State
	}
)

func (e ErrUnknownBlock) Error() string {
	return Fmt("Could not find block #%d", e.Height)
}

func (e ErrBlockHashMismatch) Error() string {
	return Fmt("App block hash (%X) does not match core block hash (%X) for height %d", e.AppHash, e.CoreHash, e.Height)
}

func (e ErrAppBlockHeightTooHigh) Error() string {
	return Fmt("App block height (%d) is higher than core (%d)", e.AppHeight, e.CoreHeight)
}
func (e ErrLastStateMismatch) Error() string {
	return Fmt("Latest tendermint block (%d) LastAppHash (%X) does not match app's AppHash (%X)", e.Height, e.Core, e.App)
}

func (e ErrStateMismatch) Error() string {
	return Fmt("State after replay does not match saved state. Got ----\n%v\nExpected ----\n%v\n", e.Got, e.Expected)
}
