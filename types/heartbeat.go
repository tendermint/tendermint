package types

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto"
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
	bz, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeHeartbeat(chainID, heartbeat))
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

// ValidateBasic performs basic validation.
func (heartbeat *Heartbeat) ValidateBasic() error {
	if len(heartbeat.ValidatorAddress) != crypto.AddressSize {
		return fmt.Errorf("Expected ValidatorAddress size to be %d bytes, got %d bytes",
			crypto.AddressSize,
			len(heartbeat.ValidatorAddress),
		)
	}
	if heartbeat.ValidatorIndex < 0 {
		return errors.New("Negative ValidatorIndex")
	}
	if heartbeat.Height < 0 {
		return errors.New("Negative Height")
	}
	if heartbeat.Round < 0 {
		return errors.New("Negative Round")
	}
	if heartbeat.Sequence < 0 {
		return errors.New("Negative Sequence")
	}
	if len(heartbeat.Signature) == 0 {
		return errors.New("Signature is missing")
	}
	if len(heartbeat.Signature) > MaxSignatureSize {
		return fmt.Errorf("Signature is too big (max: %d)", MaxSignatureSize)
	}
	return nil
}
