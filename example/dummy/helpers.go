package dummy

import (
	"github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

// RandVal creates one random validator, with a key derived
// from the input value
func RandVal(i int) *types.Validator {
	pubkey := crypto.GenPrivKeyEd25519FromSecret([]byte(cmn.Fmt("test%d", i))).PubKey().Bytes()
	power := cmn.RandUint16() + 1
	return &types.Validator{pubkey, int64(power)}
}

// RandVals returns a list of cnt validators for initializing
// the application. Note that the keys are deterministically
// derived from the index in the array, while the power is
// random (Change this if not desired)
func RandVals(cnt int) []*types.Validator {
	res := make([]*types.Validator, cnt)
	for i := 0; i < cnt; i++ {
		res[i] = RandVal(i)
	}
	return res
}

// InitDummy initializes the dummy app with some data,
// which allows tests to pass and is fine as long as you
// don't make any tx that modify the validator state
func InitDummy(app *PersistentDummyApplication) {
	app.InitChain(types.RequestInitChain{RandVals(1)})
}
