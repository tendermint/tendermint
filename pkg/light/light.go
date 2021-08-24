package light

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// LightBlock is a SignedHeader and a ValidatorSet.
// It is the basis of the light client
type LightBlock struct {
	*metadata.SignedHeader `json:"signed_header"`
	ValidatorSet           *consensus.ValidatorSet `json:"validator_set"`
}

// ValidateBasic checks that the data is correct and consistent
//
// This does no verification of the signatures
func (lb LightBlock) ValidateBasic(chainID string) error {
	if lb.SignedHeader == nil {
		return errors.New("missing signed header")
	}
	if lb.ValidatorSet == nil {
		return errors.New("missing validator set")
	}

	if err := lb.SignedHeader.ValidateBasic(chainID); err != nil {
		return fmt.Errorf("invalid signed header: %w", err)
	}
	if err := lb.ValidatorSet.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid validator set: %w", err)
	}

	// make sure the validator set is consistent with the header
	if valSetHash := lb.ValidatorSet.Hash(); !bytes.Equal(lb.SignedHeader.ValidatorsHash, valSetHash) {
		return fmt.Errorf("expected validator hash of header to match validator set hash (%X != %X)",
			lb.SignedHeader.ValidatorsHash, valSetHash,
		)
	}

	return nil
}

// String returns a string representation of the LightBlock
func (lb LightBlock) String() string {
	return lb.StringIndented("")
}

// StringIndented returns an indented string representation of the LightBlock
//
// SignedHeader
// ValidatorSet
func (lb LightBlock) StringIndented(indent string) string {
	return fmt.Sprintf(`LightBlock{
%s  %v
%s  %v
%s}`,
		indent, lb.SignedHeader.StringIndented(indent+"  "),
		indent, lb.ValidatorSet.StringIndented(indent+"  "),
		indent)
}

// ToProto converts the LightBlock to protobuf
func (lb *LightBlock) ToProto() (*tmproto.LightBlock, error) {
	if lb == nil {
		return nil, nil
	}

	lbp := new(tmproto.LightBlock)
	var err error
	if lb.SignedHeader != nil {
		lbp.SignedHeader = lb.SignedHeader.ToProto()
	}
	if lb.ValidatorSet != nil {
		lbp.ValidatorSet, err = lb.ValidatorSet.ToProto()
		if err != nil {
			return nil, err
		}
	}

	return lbp, nil
}

// LightBlockFromProto converts from protobuf back into the Lightblock.
// An error is returned if either the validator set or signed header are invalid
func LightBlockFromProto(pb *tmproto.LightBlock) (*LightBlock, error) {
	if pb == nil {
		return nil, errors.New("nil light block")
	}

	lb := new(LightBlock)

	if pb.SignedHeader != nil {
		sh, err := metadata.SignedHeaderFromProto(pb.SignedHeader)
		if err != nil {
			return nil, err
		}
		lb.SignedHeader = sh
	}

	if pb.ValidatorSet != nil {
		vals, err := consensus.ValidatorSetFromProto(pb.ValidatorSet)
		if err != nil {
			return nil, err
		}
		lb.ValidatorSet = vals
	}

	return lb, nil
}
