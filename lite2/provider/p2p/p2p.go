package p2p

import (
	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/types"
)

// p2p provider uses P2P to obtain necessary information.
type p2p struct {
	chainID string
}

// New creates a new P2P provider.
func New(chainID string) (provider.Provider, error) {
	return &p2p{
		chainID: chainID,
	}, nil
}

// ChainID implements Provider.
func (p *p2p) ChainID() string {
	return p.chainID
}

// SignedHeader implements Provider.
func (p *p2p) SignedHeader(height int64) (*types.SignedHeader, error) {
	err := validateHeight(height)
	if err != nil {
		return nil, err
	}
	panic("Not implemented")
}

// ValidatorSet implements Provider.
func (p *p2p) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	err := validateHeight(height)
	if err != nil {
		return nil, err
	}
	panic("Not implemented")
}

// validateHeight validates a height
func validateHeight(height int64) error {
	if height < 0 {
		return errors.Errorf("expected height >= 0, got height %d", height)
	}
	return nil
}
