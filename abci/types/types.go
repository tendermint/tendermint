package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/jsonpb"

	types "github.com/tendermint/tendermint/proto/tendermint/types"
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

// IsUnknown returns true if Code is Unknown
func (r ResponseVerifyVoteExtension) IsUnknown() bool {
	return r.Result == ResponseVerifyVoteExtension_UNKNOWN
}

// IsOK returns true if Code is OK
func (r ResponseVerifyVoteExtension) IsOK() bool {
	return r.Result == ResponseVerifyVoteExtension_ACCEPT
}

// IsErr returns true if Code is something other than OK.
func (r ResponseVerifyVoteExtension) IsErr() bool {
	return r.Result != ResponseVerifyVoteExtension_ACCEPT
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

func RespondExtendVote(appDataToSign, appDataSelfAuthenticating []byte) ResponseExtendVote {
	return ResponseExtendVote{
		VoteExtension: &types.VoteExtension{
			AppDataToSign:             appDataToSign,
			AppDataSelfAuthenticating: appDataSelfAuthenticating,
		},
	}
}

func RespondVerifyVoteExtension(ok bool) ResponseVerifyVoteExtension {
	result := ResponseVerifyVoteExtension_REJECT
	if ok {
		result = ResponseVerifyVoteExtension_ACCEPT
	}
	return ResponseVerifyVoteExtension{
		Result: result,
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

// TxResultsToByteSlices encodes the the TxResults as a list of byte
// slices. It strips off the non-deterministic pieces of the TxResults
// so that the resulting data can be used for hash comparisons and used
// in Merkle proofs.
func TxResultsToByteSlices(r []*ExecTxResult) ([][]byte, error) {
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

func (tr *TxRecord) isIncluded() bool {
	return tr.Action == TxRecord_ADDED || tr.Action == TxRecord_UNMODIFIED
}

// IncludedTxs returns all of the TxRecords that are marked for inclusion in the
// proposed block.
func (rpp *ResponsePrepareProposal) IncludedTxs() []*TxRecord {
	trs := []*TxRecord{}
	for _, tr := range rpp.TxRecords {
		if tr.isIncluded() {
			trs = append(trs, tr)
		}
	}
	return trs
}

// RemovedTxs returns all of the TxRecords that are marked for removal from the
// mempool.
func (rpp *ResponsePrepareProposal) RemovedTxs() []*TxRecord {
	var trs []*TxRecord
	for _, tr := range rpp.TxRecords {
		if tr.Action == TxRecord_REMOVED {
			trs = append(trs, tr)
		}
	}
	return trs
}

// AddedTxs returns all of the TxRecords that are marked as added to the proposal.
func (rpp *ResponsePrepareProposal) AddedTxs() []*TxRecord {
	var trs []*TxRecord
	for _, tr := range rpp.TxRecords {
		if tr.Action == TxRecord_ADDED {
			trs = append(trs, tr)
		}
	}
	return trs
}

// Validate checks that the fields of the ResponsePrepareProposal are properly
// constructed. Validate returns an error if any of the validation checks fail.
func (rpp *ResponsePrepareProposal) Validate(maxSizeBytes int64, otxs [][]byte) error {
	if !rpp.ModifiedTx {
		// This method currently only checks the validity of the TxRecords field.
		// If ModifiedTx is false, then we can ignore the validity of the TxRecords field.
		//
		// TODO: When implementing VoteExensions, AppSignedUpdates may be modified by the application
		// and this method should be updated to validate the AppSignedUpdates.
		return nil
	}

	// TODO: this feels like a large amount allocated data. We move all the Txs into strings
	// in the map. The map will be as large as the original byte slice.
	// Is there a key we can use?
	otxsSet := make(map[string]struct{}, len(otxs))
	for _, tx := range otxs {
		otxsSet[string(tx)] = struct{}{}
	}
	ntx := map[string]struct{}{}
	var size int64
	for _, tr := range rpp.TxRecords {
		if tr.isIncluded() {
			size += int64(len(tr.Tx))
			if size > maxSizeBytes {
				return fmt.Errorf("transaction data size %d exceeds maximum %d", size, maxSizeBytes)
			}
		}
		if _, ok := ntx[string(tr.Tx)]; ok {
			return errors.New("TxRecords contains duplicate transaction")
		}
		ntx[string(tr.Tx)] = struct{}{}
		if _, ok := otxsSet[string(tr.Tx)]; ok {
			if tr.Action == TxRecord_ADDED {
				return fmt.Errorf("unmodified transaction incorrectly marked as %s", tr.Action.String())
			}
		} else {
			if tr.Action == TxRecord_REMOVED || tr.Action == TxRecord_UNMODIFIED {
				return fmt.Errorf("unmodified transaction incorrectly marked as %s", tr.Action.String())
			}
		}
		if tr.Action == TxRecord_UNKNOWN {
			return fmt.Errorf("transaction incorrectly marked as %s", tr.Action.String())
		}
	}
	return nil
}
