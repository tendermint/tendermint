package types

import (
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// Heartbeat is a simple vote-like structure so validators can
// alert others that they are alive and waiting for transactions.
// Note: We aren't adding ",omitempty" to Heartbeat's
// json field tags because we always want the JSON
// representation to be in its canonical form.
type Heartbeat struct {
	ValidatorAddress Address `json:"validator_address"`
	ValidatorIndex   int     `json:"validator_index"`
	Height           int64   `json:"height"`
	Round            int     `json:"round"`
	Sequence         int     `json:"sequence"`
	Signature        []byte  `json:"signature"`
}

// SignBytes returns the Heartbeat bytes for signing.
// It panics if the Heartbeat is nil.
func (heartbeat *Heartbeat) SignBytes(chainID string) []byte {
	bz, err := cdc.MarshalJSON(CanonicalHeartbeat(chainID, heartbeat))
	if err != nil {
		panic(err)
	}
	return bz
}

// Copy makes a copy of the Heartbeat.
func (heartbeat *Heartbeat) Copy() *Heartbeat {
	if heartbeat == nil {
		return nil
	}
	heartbeatCopy := *heartbeat
	return &heartbeatCopy
}

// String returns a string representation of the Heartbeat.
func (heartbeat *Heartbeat) String() string {
	if heartbeat == nil {
		return "nil-heartbeat"
	}

	return fmt.Sprintf("Heartbeat{%v:%X %v/%02d (%v) %v}",
		heartbeat.ValidatorIndex, cmn.Fingerprint(heartbeat.ValidatorAddress),
		heartbeat.Height, heartbeat.Round, heartbeat.Sequence,
		fmt.Sprintf("/%X.../", cmn.Fingerprint(heartbeat.Signature[:])))
}
