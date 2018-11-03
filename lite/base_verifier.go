package lite

import (
	"bytes"

	cmn "github.com/tendermint/tendermint/libs/common"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

var _ Verifier = (*BaseVerifier)(nil)

// BaseVerifier lets us check the validity of SignedHeaders at height or
// later, requiring sufficient votes (> 2/3) from the given valset.
// To verify blocks produced by a blockchain with mutable validator sets,
// use the DynamicVerifier.
// TODO: Handle unbonding time.
type BaseVerifier struct {
	chainID string
	height  int64
	valset  *types.ValidatorSet
}

// NewBaseVerifier returns a new Verifier initialized with a validator set at
// some height.
func NewBaseVerifier(chainID string, height int64, valset *types.ValidatorSet) *BaseVerifier {
	if valset.IsNilOrEmpty() {
		panic("NewBaseVerifier requires a valid valset")
	}
	return &BaseVerifier{
		chainID: chainID,
		height:  height,
		valset:  valset,
	}
}

// Implements Verifier.
func (bc *BaseVerifier) ChainID() string {
	return bc.chainID
}

// Implements Verifier.
func (bc *BaseVerifier) Verify(signedHeader types.SignedHeader) error {

	// We can't verify commits older than bc.height.
	if signedHeader.Height < bc.height {
		return cmn.NewError("BaseVerifier height is %v, cannot verify height %v",
			bc.height, signedHeader.Height)
	}

	// We can't verify with the wrong validator set.
	if !bytes.Equal(signedHeader.ValidatorsHash,
		bc.valset.Hash()) {
		return lerr.ErrUnexpectedValidators(signedHeader.ValidatorsHash, bc.valset.Hash())
	}

	// Do basic sanity checks.
	err := signedHeader.ValidateBasic(bc.chainID)
	if err != nil {
		return cmn.ErrorWrap(err, "in verify")
	}

	// Check commit signatures.
	err = bc.valset.VerifyCommit(
		bc.chainID, signedHeader.Commit.BlockID,
		signedHeader.Height, signedHeader.Commit)
	if err != nil {
		return cmn.ErrorWrap(err, "in verify")
	}

	return nil
}
