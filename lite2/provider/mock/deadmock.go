package mock

import (
	"errors"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/types"
)

type deadMock struct {
	chainID string
}

// NewDeadMock creates a mock provider that always errors.
func NewDeadMock(chainID string) provider.Provider {
	return &deadMock{chainID: chainID}
}

func (p *deadMock) ChainID() string {
	return p.chainID
}

func (p *deadMock) String() string {
	return "deadMock"
}

func (p *deadMock) SignedHeader(height int64) (*types.SignedHeader, error) {
	return nil, errors.New("no response from provider")
}

func (p *deadMock) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	return nil, errors.New("no response from provider")
}
