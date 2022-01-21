package provider

import (
	"context"

	"github.com/tendermint/tendermint/types"
)

//go:generate ../../scripts/mockery_generate.sh Provider

// Provider provides information for the light client to sync (verification
// happens in the client).
type Provider interface {
	// LightBlock returns the LightBlock that corresponds to the given
	// height.
	//
	// 0 - the latest.
	// height must be >= 0.
	//
	// If the provider fails to fetch the LightBlock due to the IO or other
	// issues, an error will be returned.
	// If there's no LightBlock for the given height, ErrLightBlockNotFound
	// error is returned.
	LightBlock(ctx context.Context, height int64) (*types.LightBlock, error)

	// ReportEvidence reports an evidence of misbehavior.
	ReportEvidence(context.Context, types.Evidence) error

	// Returns the ID of a provider. For RPC providers it returns the IP address of the client
	// For p2p providers it returns a combination of NodeID and IP address
	ID() string
}
