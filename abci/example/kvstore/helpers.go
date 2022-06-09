package kvstore

import (
	"context"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/llmq"
	tmtypes "github.com/tendermint/tendermint/types"
)

// RandValidatorSetUpdate returns a list of cnt validators for initializing
// the application. Note that the keys are deterministically
// derived from the index in the array
func RandValidatorSetUpdate(cnt int) types.ValidatorSetUpdate {
	ld := llmq.MustGenerate(crypto.RandProTxHashes(cnt))
	vsu, err := types.LLMQToValidatorSetProto(
		*ld,
		types.WithNodeAddrs(randNodeAddrs(cnt)),
		types.WithRandQuorumHash(),
	)
	if err != nil {
		panic(err)
	}
	return *vsu
}

// InitKVStore initializes the kvstore app with some data,
// which allows tests to pass and is fine as long as you
// don't make any tx that modify the validator state
func InitKVStore(ctx context.Context, app *PersistentKVStoreApplication) error {
	val := RandValidatorSetUpdate(1)
	_, err := app.InitChain(ctx, &types.RequestInitChain{
		ValidatorSet: &val,
	})
	return err
}

func randNodeAddrs(n int) []string {
	addrs := make([]string, n)
	for i := 0; i < n; i++ {
		addrs[i] = tmtypes.RandValidatorAddress().String()
	}
	return addrs
}
