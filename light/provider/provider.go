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

	// Suggestion to rename this to ID as it should return the Primary ID (URL for RPC and ID + URL for P2P)
	// Renaming it to ID causes changes in other packages (internal/statesync) as BlockProvider also implements this interface
	// I will leave the signature to be String - this way we do not change BlockProvider
	String() string //Rename to ID - not clear what the actual string is ; The goal is to determine based on this whether we have an RPC or p2p connecetion
}
