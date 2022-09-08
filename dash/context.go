package dash

import (
	"context"
	"errors"

	"github.com/tendermint/tendermint/crypto"
)

type contextKey string

var (
	proTxHashKey contextKey = "proTxHash"
)

// ContextWithProTxHash puts node pro-tx-hash into a context
func ContextWithProTxHash(ctx context.Context, proTxHash crypto.ProTxHash) context.Context {
	return context.WithValue(ctx, proTxHashKey, proTxHash)
}

// ProTxHashFromContext retrieves node pro-tx-hash from a context
func ProTxHashFromContext(ctx context.Context) (crypto.ProTxHash, error) {
	val := ctx.Value(proTxHashKey)
	proTxHash, ok := val.(crypto.ProTxHash)
	if !ok {
		return nil, errors.New("proTxHash not found")
	}
	return proTxHash, nil
}
