package kvstore

import (
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
)

func ValUpdate(pubKey crypto.PubKey, proTxHash crypto.ProTxHash) types.ValidatorUpdate {
	return types.UpdateValidator(proTxHash, pubKey.Bytes(), 100)
}

// RandValidatorSetUpdate returns a list of cnt validators for initializing
// the application. Note that the keys are deterministically
// derived from the index in the array
func RandValidatorSetUpdate(cnt int) types.ValidatorSetUpdate {
	res := make([]types.ValidatorUpdate, cnt)

	privKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQDataDefaultThreshold(cnt)
	for i := 0; i < cnt; i++ {
		res[i] = ValUpdate(privKeys[i].PubKey(), proTxHashes[i])
	}
	thresholdPublicKeyABCI, err := cryptoenc.PubKeyToProto(thresholdPublicKey)
	if err != nil {
		panic(err)
	}
	return types.ValidatorSetUpdate{ValidatorUpdates: res, ThresholdPublicKey: thresholdPublicKeyABCI}
}

// InitKVStore initializes the kvstore app with some data,
// which allows tests to pass and is fine as long as you
// don't make any tx that modify the validator state
func InitKVStore(app *PersistentKVStoreApplication) {
	app.InitChain(types.RequestInitChain{
		ValidatorSet: RandValidatorSetUpdate(1),
	})
}
