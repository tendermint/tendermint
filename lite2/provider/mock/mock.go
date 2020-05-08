package mock

import (
	"fmt"
	"strings"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/types"
)

type Mock struct {
	chainID          string
	headers          map[int64]*types.SignedHeader
	vals             map[int64]*types.ValidatorSet
	evidenceToReport map[string]types.Evidence // hash => evidence
}

var _ provider.Provider = (*Mock)(nil)

// New creates a mock provider with the given set of headers and validator
// sets.
func New(chainID string, headers map[int64]*types.SignedHeader, vals map[int64]*types.ValidatorSet) *Mock {
	return &Mock{
		chainID:          chainID,
		headers:          headers,
		vals:             vals,
		evidenceToReport: make(map[string]types.Evidence),
	}
}

// ChainID returns the blockchain ID.
func (p *Mock) ChainID() string {
	return p.chainID
}

func (p *Mock) String() string {
	var headers strings.Builder
	for _, h := range p.headers {
		fmt.Fprintf(&headers, " %d:%X", h.Height, h.Hash())
	}

	var vals strings.Builder
	for _, v := range p.vals {
		fmt.Fprintf(&vals, " %X", v.Hash())
	}

	return fmt.Sprintf("Mock{headers: %s, vals: %v}", headers.String(), vals.String())
}

func (p *Mock) SignedHeader(height int64) (*types.SignedHeader, error) {
	if height == 0 && len(p.headers) > 0 {
		return p.headers[int64(len(p.headers))], nil
	}
	if _, ok := p.headers[height]; ok {
		return p.headers[height], nil
	}
	return nil, provider.ErrSignedHeaderNotFound
}

func (p *Mock) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	if height == 0 && len(p.vals) > 0 {
		return p.vals[int64(len(p.vals))], nil
	}
	if _, ok := p.vals[height]; ok {
		return p.vals[height], nil
	}
	return nil, provider.ErrValidatorSetNotFound
}

func (p *Mock) ReportEvidence(ev types.Evidence) error {
	p.evidenceToReport[string(ev.Hash())] = ev
	return nil
}

func (p *Mock) HasEvidence(ev types.Evidence) bool {
	_, ok := p.evidenceToReport[string(ev.Hash())]
	return ok
}
