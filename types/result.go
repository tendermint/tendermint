package types

import (
	"fmt"
)

// type CodeType uint32

const (
	CodeTypeOK uint32 = 0
)

// IsErr returns true if Code is something other than OK.
func (r ResponseCheckTx) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseCheckTx) Error() string {
	return fmtError(r.Code, r.Log)
}

// IsErr returns true if Code is something other than OK.
func (r ResponseDeliverTx) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseDeliverTx) Error() string {
	return fmtError(r.Code, r.Log)
}

// IsErr returns true if Code is something other than OK.
func (r ResponseCommit) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseCommit) Error() string {
	return fmtError(r.Code, r.Log)
}

// IsErr returns true if Code is something other than OK.
func (r ResponseQuery) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseQuery) Error() string {
	return fmtError(r.Code, r.Log)
}

func fmtError(code uint32, log string) string {
	return fmt.Sprintf("Error code (%d): %s", code, log)
}
