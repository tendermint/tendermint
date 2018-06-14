package proxy

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/lite"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

func ValidateBlockMeta(meta *types.BlockMeta, sh lite.SignedHeader) error {
	if meta == nil {
		return errors.New("expecting a non-nil BlockMeta")
	}
	// TODO: check the BlockID??
	return ValidateHeader(meta.Header, sh)
}

func ValidateBlock(meta *types.Block, sh lite.SignedHeader) error {
	if meta == nil {
		return errors.New("expecting a non-nil Block")
	}
	err := ValidateHeader(meta.Header, sh)
	if err != nil {
		return err
	}
	if !bytes.Equal(meta.Data.Hash(), meta.Header.DataHash) {
		return errors.New("Data hash doesn't match header")
	}
	return nil
}

func ValidateHeader(head *types.Header, sh lite.SignedHeader) error {
	if head == nil {
		return errors.New("expecting a non-nil Header")
	}
	// Make sure they are for the same height (obvious fail).
	if head.Height != sh.Height() {
		return lerr.ErrHeightMismatch(head.Height, sh.Height())
	}
	// Check if they are equal by using hashes.
	if !bytes.Equal(sh.Header.Hash(), sh.Header.Hash()) {
		return errors.New("Headers don't match")
	}
	return nil
}
