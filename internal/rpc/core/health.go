package core

import (
	"context"

	"github.com/tendermint/tendermint/rpc/coretypes"
)

// Health gets node health. Returns empty result (200 OK) on success, no
// response - in case of an error.
// More: https://docs.tendermint.com/master/rpc/#/Info/health
func (env *Environment) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	return &coretypes.ResultHealth{}, nil
}
