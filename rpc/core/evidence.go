package core

import (
	"github.com/pkg/errors"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// BroadcastEvidence broadcasts evidence of the misbehavior.
// More: https://docs.tendermint.com/master/rpc/#/Info/broadcast_evidence
func BroadcastEvidence(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	if err := ev.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "evidence.ValidateBasic failed")
	}

	if err := evidencePool.AddEvidence(ev); err != nil {
		return nil, err
	}

	return &ctypes.ResultBroadcastEvidence{Hash: ev.Hash()}, nil
}
