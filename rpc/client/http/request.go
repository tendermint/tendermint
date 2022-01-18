package http

// The types in this file define the JSON encoding for RPC method parameters
// from the client to the server.

import (
	"encoding/json"

	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

type abciQueryArgs struct {
	Path   string         `json:"path"`
	Data   bytes.HexBytes `json:"data"`
	Height int64          `json:"height,string"`
	Prove  bool           `json:"prove"`
}

type txArgs struct {
	Tx []byte `json:"tx"`
}

type txKeyArgs struct {
	TxKey []byte `json:"tx_key"`
}

type unconfirmedArgs struct {
	Page    *int `json:"page,string,omitempty"`
	PerPage *int `json:"per_page,string,omitempty"`
}

type heightArgs struct {
	Height *int64 `json:"height,string,omitempty"`
}

type hashArgs struct {
	Hash  bytes.HexBytes `json:"hash"`
	Prove bool           `json:"prove,omitempty"`
}

type blockchainInfoArgs struct {
	MinHeight int64 `json:"minHeight,string"`
	MaxHeight int64 `json:"maxHeight,string"`
}

type genesisChunkArgs struct {
	Chunk uint `json:"chunk,string"`
}

type searchArgs struct {
	Query   string `json:"query"`
	Prove   bool   `json:"prove,omitempty"`
	OrderBy string `json:"order_by,omitempty"`
	Page    *int   `json:"page,string,omitempty"`
	PerPage *int   `json:"per_page,string,omitempty"`
}

type validatorArgs struct {
	Height  *int64 `json:"height,string,omitempty"`
	Page    *int   `json:"page,string,omitempty"`
	PerPage *int   `json:"per_page,string,omitempty"`
}

type evidenceArgs struct {
	Evidence types.Evidence
}

// MarshalJSON implements json.Marshaler to encode the evidence using the
// wrapped concrete type of the implementation.
func (e evidenceArgs) MarshalJSON() ([]byte, error) {
	ev, err := jsontypes.Marshal(e.Evidence)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Evidence json.RawMessage `json:"evidence"`
	}{Evidence: ev})
}
