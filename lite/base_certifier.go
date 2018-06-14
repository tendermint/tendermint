package lite

import (
	"bytes"

	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var _ Certifier = (*BaseCertifier)(nil)

// BaseCertifier lets us check the validity of SignedHeaders at height or
// later, requiring sufficient votes (> 2/3) from the given valset.
// To certify blocks produced by a blockchain with mutable validator sets,
// use the InquiringCertifier.
// TODO: Handle unbonding time.
type BaseCertifier struct {
	chainID string
	height  int64
	valset  *types.ValidatorSet
}

// NewBaseCertifier returns a new certifier initialized with a validator set at
// some height.
func NewBaseCertifier(chainID string, height int64, valset *types.ValidatorSet) *BaseCertifier {
	if valset == nil || len(valset.Hash()) == 0 {
		panic("NewBaseCertifier requires a valid valset")
	}
	return &BaseCertifier{
		chainID: chainID,
		height:  height,
		valset:  valset,
	}
}

// Implements Certifier.
func (bc *BaseCertifier) ChainID() string {
	return bc.chainID
}

// Implements Certifier.
func (bc *BaseCertifier) Certify(signedHeader types.SignedHeader) error {

	// We can't certify commits older than bc.height.
	if signedHeader.Height < bc.height {
		return cmn.NewError("BaseCertifier height is %v, cannot certify height %v",
			bc.height, signedHeader.Height)
	}

	// We can't certify with the wrong validator set.
	if !bytes.Equal(signedHeader.ValidatorsHash,
		bc.valset.Hash()) {
		return lerr.ErrUnexpectedValidators(signedHeader.ValidatorsHash, bc.valset.Hash())
	}

	// Do basic sanity checks.
	err := signedHeader.ValidateBasic(bc.chainID)
	if err != nil {
		return cmn.ErrorWrap(err, "in certify")
	}

	// Check commit signatures.
	err = bc.valset.VerifyCommit(
		bc.chainID, signedHeader.Commit.BlockID,
		signedHeader.Height, signedHeader.Commit)
	if err != nil {
		return cmn.ErrorWrap(err, "in certify")
	}

	return nil
}
