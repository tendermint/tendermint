package core

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/rpc/coretypes"
)

// BroadcastEvidence broadcasts evidence of the misbehavior.
// More: https://docs.tendermint.com/master/rpc/#/Evidence/broadcast_evidence
func (env *Environment) BroadcastEvidence(ctx context.Context, req *coretypes.RequestBroadcastEvidence) (*coretypes.ResultBroadcastEvidence, error) {
	if req.Evidence == nil {
		return nil, fmt.Errorf("%w: no evidence was provided", coretypes.ErrInvalidRequest)
	}
	if err := req.Evidence.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("evidence.ValidateBasic failed: %w", err)
	}
	if err := env.EvidencePool.AddEvidence(ctx, req.Evidence); err != nil {
		return nil, fmt.Errorf("failed to add evidence: %w", err)
	}
	return &coretypes.ResultBroadcastEvidence{Hash: req.Evidence.Hash()}, nil
}
