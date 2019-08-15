package lite

import (
	"bytes"

	"github.com/pkg/errors"

	cmn "github.com/tendermint/tendermint/libs/common"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

// verifier lets us check the validity of SignedHeaders at the given height or later,
// requiring sufficient votes (> 2/3) from the given valset. To verify blocks
// produced by a blockchain with mutable validator sets, use the
// DynamicVerifier.
type verifier struct {
	chainID string
	height  int64
	valset  *types.ValidatorSet
}

// NewVerifier returns a new Verifier initialized with a validator set at
// some height.
func NewVerifier(chainID string, height int64, valset *types.ValidatorSet) *verifier {
	if valset.IsNilOrEmpty() {
		panic("NewVerifier requires a valid valset")
	}
	return &verifier{
		chainID: chainID,
		height:  height,
		valset:  valset,
	}
}

func (bv *verifier) ChainID() string {
	return bv.chainID
}

func (bv *verifier) Verify(signedHeader types.SignedHeader) error {
	// We can't verify commits for a different chain.
	if signedHeader.ChainID != bv.chainID {
		return cmn.NewError("verifier chainID is %v, cannot verify chainID %v",
			bv.chainID, signedHeader.ChainID)
	}

	// We can't verify commits older than bv.height.
	if signedHeader.Height < bv.height {
		return cmn.NewError("verifier height is %v, cannot verify height %v",
			bv.height, signedHeader.Height)
	}

	// We can't verify with the wrong validator set.
	if !bytes.Equal(signedHeader.ValidatorsHash, bv.valset.Hash()) {
		return lerr.ErrUnexpectedValidators(signedHeader.ValidatorsHash, bv.valset.Hash())
	}

	// Do basic sanity checks.
	err := signedHeader.ValidateBasic(bv.chainID)
	if err != nil {
		return errors.Wrap(err, "in verify")
	}

	// Check commit signatures.
	err = bv.valset.VerifyCommit(
		bv.chainID, signedHeader.Commit.BlockID,
		signedHeader.Height, signedHeader.Commit)
	if err != nil {
		return errors.Wrap(err, "in verify")
	}

	return nil
}
