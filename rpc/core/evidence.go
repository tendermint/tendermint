package core

import (
	"fmt"

	pbtypes "github.com/tendermint/tendermint/proto/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// BroadcastEvidence broadcasts evidence of the misbehavior. It takes the Protobuf variant
// of Evidence as input, since RPC uses the Protobuf JSON unmarshaller.
// More: https://docs.tendermint.com/master/rpc/#/Info/broadcast_evidence
func BroadcastEvidence(ctx *rpctypes.Context, pbev pbtypes.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	ev, err := types.EvidenceFromProto(pbev)
	if err != nil {
		return nil, fmt.Errorf("evidence decoding failed: %w", err)
	}

	if err := ev.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("evidence.ValidateBasic failed: %w", err)
	}

	if err := evidencePool.AddEvidence(ev); err != nil {
		return nil, fmt.Errorf("failed to add evidence: %w", err)
	}
	return &ctypes.ResultBroadcastEvidence{Hash: ev.Hash()}, nil
}
