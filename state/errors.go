package state

import (
	. "github.com/tendermint/go-common"
)

type (
	ErrInvalidBlock error
	ErrProxyAppConn error

	ErrUnknownBlock struct {
		height int
	}

	ErrBlockHashMismatch struct {
		coreHash []byte
		appHash  []byte
		height   int
	}

	ErrAppBlockHeightTooHigh struct {
		coreHeight int
		appHeight  int
	}

	ErrLastStateMismatch struct {
		height int
		core   []byte
		app    []byte
	}

	ErrStateMismatch struct {
		got      *State
		expected *State
	}
)

func (e ErrUnknownBlock) Error() string {
	return Fmt("Could not find block #%d", e.height)
}

func (e ErrBlockHashMismatch) Error() string {
	return Fmt("App block hash (%X) does not match core block hash (%X) for height %d", e.appHash, e.coreHash, e.height)
}

func (e ErrAppBlockHeightTooHigh) Error() string {
	return Fmt("App block height (%d) is higher than core (%d)", e.appHeight, e.coreHeight)
}
func (e ErrLastStateMismatch) Error() string {
	return Fmt("Latest tendermint block (%d) LastAppHash (%X) does not match app's AppHash (%X)", e.height, e.core, e.app)
}

func (e ErrStateMismatch) Error() string {
	return Fmt("State after replay does not match saved state. Got ----\n%v\nExpected ----\n%v\n", e.got, e.expected)
}
