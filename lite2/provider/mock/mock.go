package mock

import (
	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/types"
)

// mock provider allows to directly set headers & vals, which can be handy when
// testing.
type mock struct {
	chainID string
	headers map[int64]*types.SignedHeader
	vals    map[int64]*types.ValidatorSet
}

// New creates a mock provider.
func New(chainID string, headers map[int64]*types.SignedHeader, vals map[int64]*types.ValidatorSet) provider.Provider {
	return &mock{
		chainID: chainID,
		headers: headers,
		vals:    vals,
	}
}

func (p *mock) ChainID() string {
	return p.chainID
}

func (p *mock) SignedHeader(height int64) (*types.SignedHeader, error) {
	if height == 0 && len(p.headers) > 0 {
		return p.headers[int64(len(p.headers))], nil
	}
	if _, ok := p.headers[height]; ok {
		return p.headers[height], nil
	}
	return nil, provider.ErrSignedHeaderNotFound
}

func (p *mock) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	if height == 0 && len(p.vals) > 0 {
		return p.vals[int64(len(p.vals))], nil
	}
	if _, ok := p.vals[height]; ok {
		return p.vals[height], nil
	}
	return nil, provider.ErrValidatorSetNotFound
}
