package store

import "github.com/tendermint/tendermint/types"

// Store is anything that can persistenly store headers.
type Store interface {
	// SaveSignedHeaderAndValidatorSet saves a SignedHeader (h: sh.Height) and a
	// ValidatorSet (h: sh.Height).
	//
	// height must be > 0.
	SaveSignedHeaderAndValidatorSet(sh *types.SignedHeader, valSet *types.ValidatorSet) error

	// DeleteSignedHeaderAndValidatorSet deletes SignedHeader (h: height) and
	// ValidatorSet (h: height).
	//
	// height must be > 0.
	DeleteSignedHeaderAndValidatorSet(height int64) error

	// SignedHeader returns the SignedHeader that corresponds to the given
	// height.
	//
	// height must be > 0.
	//
	// If SignedHeader is not found, ErrSignedHeaderNotFound is returned.
	SignedHeader(height int64) (*types.SignedHeader, error)

	// ValidatorSet returns the ValidatorSet that corresponds to height.
	//
	// height must be > 0.
	//
	// If ValidatorSet is not found, ErrValidatorSetNotFound is returned.
	ValidatorSet(height int64) (*types.ValidatorSet, error)

	// LastSignedHeaderHeight returns the last (newest) SignedHeader height.
	//
	// If the store is empty, -1 and nil error are returned.
	LastSignedHeaderHeight() (int64, error)

	// FirstSignedHeaderHeight returns the first (oldest) SignedHeader height.
	//
	// If the store is empty, -1 and nil error are returned.
	FirstSignedHeaderHeight() (int64, error)

	// SignedHeaderBefore returns the SignedHeader before a certain height.
	//
	// height must be > 0 && <= LastSignedHeaderHeight.
	SignedHeaderBefore(height int64) (*types.SignedHeader, error)

	// Prune removes headers & the associated validator sets when Store reaches a
	// defined size (number of header & validator set pairs).
	Prune(size uint16) error

	// Size returns a number of currently existing header & validator set pairs.
	Size() uint16
}
