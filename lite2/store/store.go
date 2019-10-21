package store

import "github.com/tendermint/tendermint/types"

// Store is anything that can persistenly store headers.
type Store interface {
	// SaveSignedHeader saves a SignedHeader.
	//
	// height must be > 0.
	SaveSignedHeader(sh *types.SignedHeader) error

	// SaveValidatorSet saves a ValidatorSet.
	//
	// height must be > 0.
	SaveValidatorSet(valSet *types.ValidatorSet, height int64) error

	// SignedHeader returns the SignedHeader that corresponds to the given
	// height.
	//
	// height must be > 0.
	//
	// If the store is empty and the latest SignedHeader is requested, an error
	// is returned.
	SignedHeader(height int64) (*types.SignedHeader, error)

	// ValidatorSet returns the ValidatorSet that corresponds to height.
	//
	// height must be > 0.
	//
	// If the store is empty and the latest ValidatorSet is requested, an error
	// is returned.
	ValidatorSet(height int64) (*types.ValidatorSet, error)

	// LastSignedHeaderHeight returns the last SignedHeader height.
	//
	// If the store is empty, an error is returned.
	LastSignedHeaderHeight() (int64, error)
}
