package state

import "fmt"

type (
	ErrInvalidBlock error
	ErrProxyAppConn error

	ErrUnknownBlock struct {
		Height int64
	}

	ErrBlockHashMismatch struct {
		CoreHash []byte
		AppHash  []byte
		Height   int64
	}

	ErrAppBlockHeightTooHigh struct {
		CoreHeight int64
		AppHeight  int64
	}

	ErrLastStateMismatch struct {
		Height int64
		Core   []byte
		App    []byte
	}

	ErrStateMismatch struct {
		Got      *State
		Expected *State
	}

	ErrNoValSetForHeight struct {
		Height int64
	}

	ErrNoConsensusParamsForHeight struct {
		Height int64
	}

	ErrNoABCIResponsesForHeight struct {
		Height int64
	}
)

func (e ErrUnknownBlock) Error() string {
	return fmt.Sprintf("Could not find block #%d", e.Height)
}

func (e ErrBlockHashMismatch) Error() string {
	return fmt.Sprintf("App block hash (%X) does not match core block hash (%X) for height %d", e.AppHash, e.CoreHash, e.Height)
}

func (e ErrAppBlockHeightTooHigh) Error() string {
	return fmt.Sprintf("App block height (%d) is higher than core (%d)", e.AppHeight, e.CoreHeight)
}
func (e ErrLastStateMismatch) Error() string {
	return fmt.Sprintf("Latest tendermint block (%d) LastAppHash (%X) does not match app's AppHash (%X)", e.Height, e.Core, e.App)
}

func (e ErrStateMismatch) Error() string {
	return fmt.Sprintf("State after replay does not match saved state. Got ----\n%v\nExpected ----\n%v\n", e.Got, e.Expected)
}

func (e ErrNoValSetForHeight) Error() string {
	return fmt.Sprintf("Could not find validator set for height #%d", e.Height)
}

func (e ErrNoConsensusParamsForHeight) Error() string {
	return fmt.Sprintf("Could not find consensus params for height #%d", e.Height)
}

func (e ErrNoABCIResponsesForHeight) Error() string {
	return fmt.Sprintf("Could not find results for height #%d", e.Height)
}
