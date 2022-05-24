package types

import (
	"bytes"
	"encoding/json"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	tmjson "github.com/tendermint/tendermint/libs/json"
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

// IsOK returns true if Code is OK.
func (r ResponseDeliverTx) IsOK() bool {
	return r.Code == CodeTypeOK
}

// IsErr returns true if Code is something other than OK.
func (r ResponseDeliverTx) IsErr() bool {
	return r.Code != CodeTypeOK
}

// IsOK returns true if Code is OK.
func (r ResponseQuery) IsOK() bool {
	return r.Code == CodeTypeOK
}

// IsErr returns true if Code is something other than OK.
func (r ResponseQuery) IsErr() bool {
	return r.Code != CodeTypeOK
}

//---------------------------------------------------------------------------
// override JSON marshaling so we emit defaults (ie. disable omitempty)

var (
	jsonpbMarshaller = jsonpb.Marshaler{
		EnumsAsInts:  true,
		EmitDefaults: true,
	}
	jsonpbUnmarshaller = jsonpb.Unmarshaler{}
)

func (r *ResponseCheckTx) MarshalJSON() ([]byte, error) {
	s, err := jsonpbMarshaller.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseCheckTx) UnmarshalJSON(b []byte) error {
	reader := bytes.NewBuffer(b)
	return jsonpbUnmarshaller.Unmarshal(reader, r)
}

func (r *ResponseDeliverTx) MarshalJSON() ([]byte, error) {
	s, err := jsonpbMarshaller.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseDeliverTx) UnmarshalJSON(b []byte) error {
	reader := bytes.NewBuffer(b)
	return jsonpbUnmarshaller.Unmarshal(reader, r)
}

func (r *ResponseQuery) MarshalJSON() ([]byte, error) {
	s, err := jsonpbMarshaller.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseQuery) UnmarshalJSON(b []byte) error {
	reader := bytes.NewBuffer(b)
	return jsonpbUnmarshaller.Unmarshal(reader, r)
}

func (r *ResponseCommit) MarshalJSON() ([]byte, error) {
	s, err := jsonpbMarshaller.MarshalToString(r)
	return []byte(s), err
}

func (r *ResponseCommit) UnmarshalJSON(b []byte) error {
	reader := bytes.NewBuffer(b)
	return jsonpbUnmarshaller.Unmarshal(reader, r)
}

func (r *EventAttribute) MarshalJSON() ([]byte, error) {
	s, err := jsonpbMarshaller.MarshalToString(r)
	return []byte(s), err
}

func (r *EventAttribute) UnmarshalJSON(b []byte) error {
	reader := bytes.NewBuffer(b)
	return jsonpbUnmarshaller.Unmarshal(reader, r)
}

// validatorUpdateJSON is the JSON encoding of a validator update.
//
// It handles translation of public keys from the protobuf representation to
// the legacy Amino-compatible format expected by RPC clients.
type validatorUpdateJSON struct {
	PubKey json.RawMessage `json:"pub_key,omitempty"`
	Power  int64           `json:"power,string"`
}

func (v *ValidatorUpdate) MarshalJSON() ([]byte, error) {
	key, err := encoding.PubKeyFromProto(v.PubKey)
	if err != nil {
		return nil, err
	}
	jkey, err := tmjson.Marshal(key)
	if err != nil {
		return nil, err
	}
	return json.Marshal(validatorUpdateJSON{
		PubKey: jkey,
		Power:  v.GetPower(),
	})
}

func (v *ValidatorUpdate) UnmarshalJSON(data []byte) error {
	var vu validatorUpdateJSON
	if err := json.Unmarshal(data, &vu); err != nil {
		return err
	}
	var key crypto.PubKey
	if err := tmjson.Unmarshal(vu.PubKey, &key); err != nil {
		return err
	}
	pkey, err := encoding.PubKeyToProto(key)
	if err != nil {
		return err
	}
	v.PubKey = pkey
	v.Power = vu.Power
	return nil
}

// Some compile time assertions to ensure we don't
// have accidental runtime surprises later on.

// jsonEncodingRoundTripper ensures that asserted
// interfaces implement both MarshalJSON and UnmarshalJSON
type jsonRoundTripper interface {
	json.Marshaler
	json.Unmarshaler
}

var _ jsonRoundTripper = (*ResponseCommit)(nil)
var _ jsonRoundTripper = (*ResponseQuery)(nil)
var _ jsonRoundTripper = (*ResponseDeliverTx)(nil)
var _ jsonRoundTripper = (*ResponseCheckTx)(nil)

var _ jsonRoundTripper = (*EventAttribute)(nil)
