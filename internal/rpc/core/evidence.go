package core

import (
	"fmt"

	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/types"
)

// BroadcastEvidence broadcasts evidence of the misbehavior.
// More: https://docs.tendermint.com/master/rpc/#/Evidence/broadcast_evidence
func (env *Environment) BroadcastEvidence(
	ctx *rpctypes.Context,
	ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {

	if ev == nil {
		return nil, fmt.Errorf("%w: no evidence was provided", coretypes.ErrInvalidRequest)
	}

	if err := ev.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("evidence.ValidateBasic failed: %w", err)
	}

	if err := env.EvidencePool.AddEvidence(ev); err != nil {
		return nil, fmt.Errorf("failed to add evidence: %w", err)
	}
	return &coretypes.ResultBroadcastEvidence{Hash: ev.Hash()}, nil
}
