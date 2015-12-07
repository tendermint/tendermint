package types

import (
	"errors"
)

type RetCode int

// Reserved return codes
const (
	RetCodeOK               RetCode = 0
	RetCodeInternalError    RetCode = 1
	RetCodeUnauthorized     RetCode = 2
	RetCodeInsufficientFees RetCode = 3
	RetCodeUnknownRequest   RetCode = 4
	RetCodeEncodingError    RetCode = 5
)

func (r RetCode) Error() error {
	switch r {
	case RetCodeOK:
		return nil
	default:
		return errors.New(r.String())
	}
}

//go:generate stringer -type=RetCode

// NOTE: The previous comment generates r.String().
// To run it, `go get golang.org/x/tools/cmd/stringer`
// and `go generate` in tmsp/types
