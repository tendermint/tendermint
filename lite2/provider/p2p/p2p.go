package p2p

import (
	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/reactor"
	p2pm "github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

// p2p provider uses P2P to obtain necessary information.
type p2p struct {
	dispatcher *reactor.Dispatcher
	chainID    string
	peer       p2pm.Peer
}

// New creates a new P2P provider. It takes a dispatcher from lite2/reactor.Reactor.Dispatcher()
// and an appropriate peer to communicate with.
func New(dispatcher *reactor.Dispatcher, chainID string, peer p2pm.Peer) (provider.Provider, error) {
	return &p2p{
		dispatcher: dispatcher,
		chainID:    chainID,
		peer:       peer,
	}, nil
}

// ChainID implements Provider.
func (p *p2p) ChainID() string {
	return p.chainID
}

// SignedHeader implements Provider.
func (p *p2p) SignedHeader(height int64) (*types.SignedHeader, error) {
	header, err := p.dispatcher.SignedHeader(p.peer, height)
	switch {
	case err != nil:
		return nil, err
	case header == nil:
		return nil, provider.ErrSignedHeaderNotFound
	case p.chainID != header.ChainID:
		return nil, errors.Errorf("expected chain ID %q, got %q", p.chainID, header.ChainID)
	default:
		return header, nil
	}
}

// ValidatorSet implements Provider.
func (p *p2p) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	vals, err := p.dispatcher.ValidatorSet(p.peer, height)
	switch {
	case err != nil:
		return nil, err
	case vals == nil:
		return nil, provider.ErrValidatorSetNotFound
	default:
		return vals, nil
	}
}
