package types

import (
	"fmt"
	"io"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

type Heartbeat struct {
	ValidatorAddress data.Bytes       `json:"validator_address"`
	ValidatorIndex   int              `json:"validator_index"`
	Height           int              `json:"height"`
	Round            int              `json:"round"`
	Sequence         int              `json:"sequence"`
	Signature        crypto.Signature `json:"signature"`
}

func (heartbeat *Heartbeat) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceHeartbeat{
		chainID,
		CanonicalHeartbeat(heartbeat),
	}, w, n, err)
}

func (heartbeat *Heartbeat) Copy() *Heartbeat {
	heartbeatCopy := *heartbeat
	return &heartbeatCopy
}

func (heartbeat *Heartbeat) String() string {
	if heartbeat == nil {
		return "nil-heartbeat"
	}

	return fmt.Sprintf("Heartbeat{%v:%X %v/%02d (%v) %v}",
		heartbeat.ValidatorIndex, cmn.Fingerprint(heartbeat.ValidatorAddress),
		heartbeat.Height, heartbeat.Round, heartbeat.Sequence, heartbeat.Signature)
}
