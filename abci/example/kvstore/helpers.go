package kvstore

import (
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/dash/llmq"
	tmtypes "github.com/tendermint/tendermint/types"
)

func ValUpdate(
	pubKey crypto.PubKey, proTxHash crypto.ProTxHash, address tmtypes.ValidatorAddress) types.ValidatorUpdate {
	return types.UpdateValidator(proTxHash, pubKey.Bytes(), tmtypes.DefaultDashVotingPower, address.String())
}

// RandValidatorSetUpdate returns a list of cnt validators for initializing
// the application. Note that the keys are deterministically
// derived from the index in the array
func RandValidatorSetUpdate(cnt int) types.ValidatorSetUpdate {
	res := make([]types.ValidatorUpdate, 0, cnt)
	ld := llmq.MustGenerate(crypto.RandProTxHashes(cnt))
	iter := ld.Iter()
	for iter.Next() {
		proTxHash, qks := iter.Value()
		res = append(res, ValUpdate(qks.PubKey, proTxHash, tmtypes.RandValidatorAddress()))
	}
	thresholdPublicKeyABCI, err := cryptoenc.PubKeyToProto(ld.ThresholdPubKey)
	if err != nil {
		panic(err)
	}
	return types.ValidatorSetUpdate{
		ValidatorUpdates:   res,
		ThresholdPublicKey: thresholdPublicKeyABCI,
		QuorumHash:         crypto.RandQuorumHash(),
	}
}

// InitKVStore initializes the kvstore app with some data,
// which allows tests to pass and is fine as long as you
// don't make any tx that modify the validator state
func InitKVStore(app *PersistentKVStoreApplication) {
	val := RandValidatorSetUpdate(1)
	app.InitChain(types.RequestInitChain{
		ValidatorSet: &val,
	})
}
