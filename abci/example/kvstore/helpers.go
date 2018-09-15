package kvstore

import (
	"github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

// RandVal creates one random validator, with a key derived
// from the input value
func RandVal(i int) types.ValidatorUpdate {
	pubkey := cmn.RandBytes(32)
	power := cmn.RandUint16() + 1
	v := types.Ed25519ValidatorUpdate(pubkey, int64(power))
	return v
}

// RandVals returns a list of cnt validators for initializing
// the application. Note that the keys are deterministically
// derived from the index in the array, while the power is
// random (Change this if not desired)
func RandVals(cnt int) []types.ValidatorUpdate {
	res := make([]types.ValidatorUpdate, cnt)
	for i := 0; i < cnt; i++ {
		res[i] = RandVal(i)
	}
	return res
}

// InitKVStore initializes the kvstore app with some data,
// which allows tests to pass and is fine as long as you
// don't make any tx that modify the validator state
func InitKVStore(app *PersistentKVStoreApplication) {
	app.InitChain(types.RequestInitChain{
		Validators: RandVals(1),
	})
}
