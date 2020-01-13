package store

import "github.com/tendermint/tendermint/types"

// Store is anything that can persistenly store headers.
type Store interface {
	// SaveSignedHeaderAndNextValidatorSet saves a SignedHeader (h: sh.Height)
	// and a ValidatorSet (h: sh.Height+1).
	//
	// height must be > 0.
	SaveSignedHeaderAndNextValidatorSet(sh *types.SignedHeader, valSet *types.ValidatorSet) error

	// DeleteSignedHeaderAndNextValidatorSet deletes SignedHeader (h: height) and
	// ValidatorSet (h: height+1).
	//
	// height must be > 0.
	DeleteSignedHeaderAndNextValidatorSet(height int64) error

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

	// LastSignedHeaderHeight returns the last (newest) SignedHeader height.
	//
	// If the store is empty, -1 and nil error are returned.
	LastSignedHeaderHeight() (int64, error)

	// FirstSignedHeaderHeight returns the first (oldest) SignedHeader height.
	//
	// If the store is empty, -1 and nil error are returned.
	FirstSignedHeaderHeight() (int64, error)
}
