package types

import (
	"fmt"

	"github.com/gogo/protobuf/jsonpb"
)

const (
	CodeTypeOK uint32 = 0
)

// IsOK returns true if Code is OK.
func (r ResponseCheckTx) IsOK() bool {
	return r.Code == CodeTypeOK
}

// IsErr returns true if Code is something other than OK.
func (r ResponseCheckTx) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseCheckTx) Error() string {
	return fmtError(r.Code, r.Log)
}

// IsOK returns true if Code is OK.
func (r ResponseDeliverTx) IsOK() bool {
	return r.Code == CodeTypeOK
}

// IsErr returns true if Code is something other than OK.
func (r ResponseDeliverTx) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseDeliverTx) Error() string {
	return fmtError(r.Code, r.Log)
}

// IsOK returns true if Code is OK.
func (r ResponseCommit) IsOK() bool {
	return r.Code == CodeTypeOK
}

// IsErr returns true if Code is something other than OK.
func (r ResponseCommit) IsErr() bool {
	return r.Code != CodeTypeOK
}

// Error implements error interface by formatting response as string.
func (r ResponseCommit) Error() string {
	return fmtError(r.Code, r.Log)
}

// IsOK returns true if Code is OK.
func (r ResponseQuery) IsOK() bool {
	return r.Code == CodeTypeOK
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

//---------------------------------------------------------------------------
// override JSON marshalling so we dont emit defaults (ie. disable omitempty)

func (r *ResponseSetOption) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{EmitDefaults: true}
	s, err := m.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseCheckTx) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{EmitDefaults: true}
	s, err := m.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseDeliverTx) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{EmitDefaults: true}
	s, err := m.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseQuery) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{EmitDefaults: true}
	s, err := m.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseCommit) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{EmitDefaults: true}
	s, err := m.MarshalToString(r)
	return []byte(s), err
}
