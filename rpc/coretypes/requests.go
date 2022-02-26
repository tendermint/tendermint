package coretypes

import (
	"encoding/json"
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
	MinHeight int64 `json:"minHeight,string"`
	MaxHeight int64 `json:"maxHeigh,string"`
}

type RequestGenesisChunked struct {
	Chunk int64 `json:"chunk,string"`
}

type RequestBlockInfo struct {
	Height *int64 `json:"height,string"`
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
	Page    *int   `json:"page"`
	PerPage *int   `json:"per_page"`
	OrderBy string `json:"order_by"`
}

type RequestBlockSearch struct {
	Query   string `json:"query"`
	Page    *int   `json:"page"`
	PerPage *int   `json:"per_page"`
	OrderBy string `json:"order_by"`
}

type RequestValidators struct {
	Height  *int64 `json:"height,string"`
	Page    *int   `json:"page"`
	PerPage *int   `json:"per_page"`
}

type RequestConsensusParams struct {
	Height *int64 `json:"height,string"`
}

type RequestUnconfirmedTxs struct {
	Page    *int `json:"page"`
	PerPage *int `json:"per_page"`
}

type RequestBroadcastTx struct {
	Tx types.Tx `json:"tx"`
}

type RequestABCIQuery struct {
	Path   string         `json:"path"`
	Data   bytes.HexBytes `json:"data"`
	Height int64          `json:"height,string"`
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
