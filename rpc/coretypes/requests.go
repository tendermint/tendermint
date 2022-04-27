package coretypes

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

type RequestSubscribe struct {
	Query string `json:"query"`
}

type RequestUnsubscribe struct {
	Query string `json:"query"`
}

type RequestBlockchainInfo struct {
	MinHeight Int64 `json:"minHeight"`
	MaxHeight Int64 `json:"maxHeight"`
}

type RequestGenesisChunked struct {
	Chunk Int64 `json:"chunk"`
}

type RequestBlockInfo struct {
	Height *Int64 `json:"height"`
}

type RequestBlockByHash struct {
	Hash bytes.HexBytes `json:"hash"`
}

type RequestCheckTx struct {
	Tx types.Tx `json:"tx"`
}

type RequestRemoveTx struct {
	TxKey types.TxKey `json:"txkey"`
}

type RequestTx struct {
	Hash  bytes.HexBytes `json:"hash"`
	Prove bool           `json:"prove"`
}

type RequestTxSearch struct {
	Query   string `json:"query"`
	Prove   bool   `json:"prove"`
	Page    *Int64 `json:"page"`
	PerPage *Int64 `json:"per_page"`
	OrderBy string `json:"order_by"`
}

type RequestBlockSearch struct {
	Query   string `json:"query"`
	Page    *Int64 `json:"page"`
	PerPage *Int64 `json:"per_page"`
	OrderBy string `json:"order_by"`
}

type RequestValidators struct {
	Height  *Int64 `json:"height"`
	Page    *Int64 `json:"page"`
	PerPage *Int64 `json:"per_page"`
}

type RequestConsensusParams struct {
	Height *Int64 `json:"height"`
}

type RequestUnconfirmedTxs struct {
	Page    *Int64 `json:"page"`
	PerPage *Int64 `json:"per_page"`
}

type RequestBroadcastTx struct {
	Tx types.Tx `json:"tx"`
}

type RequestABCIQuery struct {
	Path   string         `json:"path"`
	Data   bytes.HexBytes `json:"data"`
	Height Int64          `json:"height"`
	Prove  bool           `json:"prove"`
}

type RequestBroadcastEvidence struct {
	Evidence types.Evidence
}

type requestBroadcastEvidenceJSON struct {
	Evidence json.RawMessage `json:"evidence"`
}

func (r RequestBroadcastEvidence) MarshalJSON() ([]byte, error) {
	ev, err := jsontypes.Marshal(r.Evidence)
	if err != nil {
		return nil, err
	}
	return json.Marshal(requestBroadcastEvidenceJSON{
		Evidence: ev,
	})
}

func (r *RequestBroadcastEvidence) UnmarshalJSON(data []byte) error {
	var val requestBroadcastEvidenceJSON
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(val.Evidence, &r.Evidence); err != nil {
		return err
	}
	return nil
}

// RequestEvents is the argument for the "/events" RPC endpoint.
type RequestEvents struct {
	// Optional filter spec. If nil or empty, all items are eligible.
	Filter *EventFilter `json:"filter"`

	// The maximum number of eligible items to return.
	// If zero or negative, the server will report a default number.
	MaxItems int `json:"maxItems"`

	// Return only items after this cursor. If empty, the limit is just
	// before the the beginning of the event log.
	After string `json:"after"`

	// Return only items before this cursor.  If empty, the limit is just
	// after the head of the event log.
	Before string `json:"before"`

	// Wait for up to this long for events to be available.
	WaitTime time.Duration `json:"waitTime"`
}

// An EventFilter specifies which events are selected by an /events request.
type EventFilter struct {
	Query string `json:"query"`
}

// Int64 is a wrapper for int64 that encodes to JSON as a string and can be
// decoded from either a string or a number value.
type Int64 int64

func (z *Int64) UnmarshalJSON(data []byte) error {
	var s string
	if len(data) != 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
	} else {
		s = string(data)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*z = Int64(v)
	return nil
}

func (z Int64) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(z), 10)), nil
}

// IntPtr returns a pointer to the value of *z as an int, or nil if z == nil.
func (z *Int64) IntPtr() *int {
	if z == nil {
		return nil
	}
	v := int(*z)
	return &v
}

// Int64Ptr returns an *Int64 that points to the same value as v, or nil.
func Int64Ptr(v *int) *Int64 {
	if v == nil {
		return nil
	}
	z := Int64(*v)
	return &z
}
