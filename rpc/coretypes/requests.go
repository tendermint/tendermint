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
	MinHeight int64
	MaxHeight int64
}

type requestBlockchainInfoJSON struct {
	MinHeight intString `json:"minHeight"`
	MaxHeight intString `json:"maxHeight"`
}

func (r *RequestBlockchainInfo) UnmarshalJSON(data []byte) error {
	var tmp requestBlockchainInfoJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.MinHeight = int64(tmp.MinHeight)
	r.MaxHeight = int64(tmp.MaxHeight)
	return nil
}

type RequestGenesisChunked struct {
	Chunk int64
}

type requestGenesisChunkedJSON struct {
	Chunk intString `json:"chunk"`
}

func (r *RequestGenesisChunked) UnmarshalJSON(data []byte) error {
	var tmp requestGenesisChunkedJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Chunk = int64(tmp.Chunk)
	return nil
}

type RequestBlockInfo struct {
	Height *int64
}

type requestBlockInfoJSON struct {
	Height *intString `json:"height"`
}

func (r *RequestBlockInfo) UnmarshalJSON(data []byte) error {
	var tmp requestBlockInfoJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Height = (*int64)(tmp.Height)
	return nil
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
	Query   string
	Prove   bool
	Page    *int
	PerPage *int   `json:"per_page"`
	OrderBy string `json:"order_by"`
}

type requestTxSearchJSON struct {
	Query   string     `json:"query"`
	Prove   bool       `json:"prove"`
	Page    *intString `json:"page"`
	PerPage *intString `json:"per_page"`
	OrderBy string     `json:"order_by"`
}

func (r *RequestTxSearch) UnmarshalJSON(data []byte) error {
	var tmp requestTxSearchJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Query = tmp.Query
	r.Prove = tmp.Prove
	r.Page = maybeIntPtr(tmp.Page)
	r.PerPage = maybeIntPtr(tmp.PerPage)
	r.OrderBy = tmp.OrderBy
	return nil
}

type RequestBlockSearch struct {
	Query   string
	Page    *int
	PerPage *int   `json:"per_page"`
	OrderBy string `json:"order_by"`
}

type requestBlockSearchJSON struct {
	Query   string     `json:"query"`
	Page    *intString `json:"page"`
	PerPage *intString `json:"per_page"`
	OrderBy string     `json:"order_by"`
}

func (r *RequestBlockSearch) UnmarshalJSON(data []byte) error {
	var tmp requestBlockSearchJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Query = tmp.Query
	r.Page = maybeIntPtr(tmp.Page)
	r.PerPage = maybeIntPtr(tmp.PerPage)
	r.OrderBy = tmp.OrderBy
	return nil
}

type RequestValidators struct {
	Height  *int64
	Page    *int
	PerPage *int `json:"per_page"`
}

type requestValidatorsJSON struct {
	Height  *intString `json:"height"`
	Page    *intString `json:"page"`
	PerPage *intString `json:"per_page"`
}

func (r *RequestValidators) UnmarshalJSON(data []byte) error {
	var tmp requestValidatorsJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Height = (*int64)(tmp.Height)
	r.Page = maybeIntPtr(tmp.Page)
	r.PerPage = maybeIntPtr(tmp.PerPage)
	return nil
}

type RequestConsensusParams struct {
	Height *int64
}

type requestConsensusParamsJSON struct {
	Height *intString `json:"height"`
}

func (r *RequestConsensusParams) UnmarshalJSON(data []byte) error {
	var tmp requestConsensusParamsJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Height = (*int64)(tmp.Height)
	return nil
}

type RequestUnconfirmedTxs struct {
	Page    *int
	PerPage *int `json:"per_page"`
}

type requestUnconfirmedTxsJSON struct {
	Page    *intString `json:"page"`
	PerPage *intString `json:"per_page"`
}

func (r *RequestUnconfirmedTxs) UnmarshalJSON(data []byte) error {
	var tmp requestUnconfirmedTxsJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Page = maybeIntPtr(tmp.Page)
	r.PerPage = maybeIntPtr(tmp.PerPage)
	return nil
}

type RequestBroadcastTx struct {
	Tx types.Tx `json:"tx"`
}

type RequestABCIQuery struct {
	Path   string
	Data   bytes.HexBytes
	Height int64
	Prove  bool
}

type requestABCIQueryJSON struct {
	Path   string         `json:"path"`
	Data   bytes.HexBytes `json:"data"`
	Height intString      `json:"height"`
	Prove  bool           `json:"prove"`
}

func (r *RequestABCIQuery) UnmarshalJSON(data []byte) error {
	var tmp requestABCIQueryJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Path = tmp.Path
	r.Data = tmp.Data
	r.Height = int64(tmp.Height)
	r.Prove = tmp.Prove
	return nil
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

// int64String is a wrapper for int64 that encodes to JSON as a string and can
// be decoded from either a string or a number value.
type intString int64

func (z *intString) UnmarshalJSON(data []byte) error {
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
	*z = intString(v)
	return nil
}

func (z intString) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(z), 10)), nil
}

func maybeIntPtr(p *intString) *int {
	if p == nil {
		return nil
	}
	z := int(*p)
	return &z
}
