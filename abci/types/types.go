package types

import (
	"bytes"
	"encoding/json"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/jsontypes"
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
func (r ExecTxResult) IsOK() bool {
	return r.Code == CodeTypeOK
}

// IsErr returns true if Code is something other than OK.
func (r ExecTxResult) IsErr() bool {
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

func (r ResponseProcessProposal) IsAccepted() bool {
	return r.Status == ResponseProcessProposal_ACCEPT
}

func (r ResponseProcessProposal) IsStatusUnknown() bool {
	return r.Status == ResponseProcessProposal_UNKNOWN
}

// IsStatusUnknown returns true if Code is Unknown
func (r ResponseVerifyVoteExtension) IsStatusUnknown() bool {
	return r.Status == ResponseVerifyVoteExtension_UNKNOWN
}

// IsOK returns true if Code is OK
func (r ResponseVerifyVoteExtension) IsOK() bool {
	return r.Status == ResponseVerifyVoteExtension_ACCEPT
}

// IsErr returns true if Code is something other than OK.
func (r ResponseVerifyVoteExtension) IsErr() bool {
	return r.Status != ResponseVerifyVoteExtension_ACCEPT
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
	jkey, err := jsontypes.Marshal(key)
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
	if err := jsontypes.Unmarshal(vu.PubKey, &key); err != nil {
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

// -----------------------------------------------
// construct Result data

func RespondVerifyVoteExtension(ok bool) ResponseVerifyVoteExtension {
	status := ResponseVerifyVoteExtension_REJECT
	if ok {
		status = ResponseVerifyVoteExtension_ACCEPT
	}
	return ResponseVerifyVoteExtension{
		Status: status,
	}
}

// deterministicExecTxResult constructs a copy of response that omits
// non-deterministic fields. The input response is not modified.
func deterministicExecTxResult(response *ExecTxResult) *ExecTxResult {
	return &ExecTxResult{
		Code:      response.Code,
		Data:      response.Data,
		GasWanted: response.GasWanted,
		GasUsed:   response.GasUsed,
	}
}

// MarshalTxResults encodes the the TxResults as a list of byte
// slices. It strips off the non-deterministic pieces of the TxResults
// so that the resulting data can be used for hash comparisons and used
// in Merkle proofs.
func MarshalTxResults(r []*ExecTxResult) ([][]byte, error) {
	s := make([][]byte, len(r))
	for i, e := range r {
		d := deterministicExecTxResult(e)
		b, err := d.Marshal()
		if err != nil {
			return nil, err
		}
		s[i] = b
	}
	return s, nil
}
