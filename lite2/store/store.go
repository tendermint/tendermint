package store

import "github.com/tendermint/tendermint/types"

// Store is anything that can persistenly store headers.
type Store interface {
	// SaveSignedHeader saves a SignedHeader.
	SaveSignedHeader(sh *types.SignedHeader) error

	// SaveValidatorSet saves a ValidatorSet.
	SaveValidatorSet(valSet *types.ValidatorSet, height int64) error

	// SignedHeader returns the SignedHeader that corresponds to the given
	// height.
	//
	// height must be > 0.
	SignedHeader(height int64) (*types.SignedHeader, error)

	// ValidatorSet returns the ValidatorSet that corresponds to height.
	//
	// height must be > 0.
	ValidatorSet(height int64) (*types.ValidatorSet, error)
}
