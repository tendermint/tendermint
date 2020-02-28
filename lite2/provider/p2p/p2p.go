package p2p

import (
	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/reactor"
	"github.com/tendermint/tendermint/types"
)

// p2p provider uses P2P to obtain necessary information.
type p2p struct {
	chainID string
	reactor *reactor.Reactor // FIXME Maybe not claim entire reactor
}

// New creates a new P2P provider. It takes a lite client reactor.
func New(chainID string, reactor *reactor.Reactor) (provider.Provider, error) {
	return &p2p{
		chainID: chainID,
		reactor: reactor,
	}, nil
}

// ChainID implements Provider.
func (p *p2p) ChainID() string {
	return p.chainID
}

// SignedHeader implements Provider.
func (p *p2p) SignedHeader(height int64) (*types.SignedHeader, error) {
	return p.reactor.SignedHeader(height)
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
