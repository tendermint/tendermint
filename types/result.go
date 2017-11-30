package types

import (
	"fmt"

	"github.com/tendermint/go-wire/data"
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

func fmtError(code uint32, log string) string {
	return fmt.Sprintf("Error code (%d): %s", code, log)
}

// ResultQuery is a wrapper around ResponseQuery using data.Bytes instead of
// raw byte slices.
type ResultQuery struct {
	Code   uint32     `json:"code"`
	Index  int64      `json:"index"`
	Key    data.Bytes `json:"key"`
	Value  data.Bytes `json:"value"`
	Proof  data.Bytes `json:"proof"`
	Height uint64     `json:"height"`
	Log    string     `json:"log"`
}

// Result converts response query to ResultQuery.
func (r *ResponseQuery) Result() *ResultQuery {
	return &ResultQuery{
		Code:   r.Code,
		Index:  r.Index,
		Key:    r.Key,
		Value:  r.Value,
		Proof:  r.Proof,
		Height: r.Height,
		Log:    r.Log,
	}
}

// IsErr returns true if Code is something other than OK.
func (r *ResultQuery) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting result as string.
func (r *ResultQuery) Error() string {
	return fmtError(r.Code, r.Log)
}
