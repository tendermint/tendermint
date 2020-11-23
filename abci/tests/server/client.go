package testsuite

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/bls12381"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
)

func InitChain(client abcicli.Client) error {
	total := 10
	vals := make([]types.ValidatorUpdate, total)
	privKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQDataDefaultThreshold(total)
	for i := 0; i < total; i++ {
		pubkey := privKeys[i].PubKey().Bytes()
		proTxHash := proTxHashes[i]
		power := 100
		vals[i] = types.UpdateValidator(proTxHash, pubkey, int64(power))
	}
	abciThresholdPublicKey, err := cryptoenc.PubKeyToProto(thresholdPublicKey)
	if err != nil {
		return err
	}
	validatorSet := types.UpdateValidatorSet(vals, abciThresholdPublicKey)
	_, err = client.InitChainSync(types.RequestInitChain{
		ValidatorSet: validatorSet,
	})
	if err != nil {
		fmt.Printf("Failed test: InitChain - %v\n", err)
		return err
	}
	fmt.Println("Passed test: InitChain")
	return nil
}

func SetOption(client abcicli.Client, key, value string) error {
	_, err := client.SetOptionSync(types.RequestSetOption{Key: key, Value: value})
	if err != nil {
		fmt.Println("Failed test: SetOption")
		fmt.Printf("error while setting %v=%v: \nerror: %v\n", key, value, err)
		return err
	}
	fmt.Println("Passed test: SetOption")
	return nil
}

func Commit(client abcicli.Client, hashExp []byte) error {
	res, err := client.CommitSync()
	data := res.Data
	if err != nil {
		fmt.Println("Failed test: Commit")
		fmt.Printf("error while committing: %v\n", err)
		return err
	}
	if !bytes.Equal(data, hashExp) {
		fmt.Println("Failed test: Commit")
		fmt.Printf("Commit hash was unexpected. Got %X expected %X\n", data, hashExp)
		return errors.New("commitTx failed")
	}
	fmt.Println("Passed test: Commit")
	return nil
}

func DeliverTx(client abcicli.Client, txBytes []byte, codeExp uint32, dataExp []byte) error {
	res, _ := client.DeliverTxSync(types.RequestDeliverTx{Tx: txBytes})
	code, data, log := res.Code, res.Data, res.Log
	if code != codeExp {
		fmt.Println("Failed test: DeliverTx")
		fmt.Printf("DeliverTx response code was unexpected. Got %v expected %v. Log: %v\n",
			code, codeExp, log)
		return errors.New("deliverTx error")
	}
	if !bytes.Equal(data, dataExp) {
		fmt.Println("Failed test: DeliverTx")
		fmt.Printf("DeliverTx response data was unexpected. Got %X expected %X\n",
			data, dataExp)
		return errors.New("deliverTx error")
	}
	fmt.Println("Passed test: DeliverTx")
	return nil
}

func CheckTx(client abcicli.Client, txBytes []byte, codeExp uint32, dataExp []byte) error {
	res, _ := client.CheckTxSync(types.RequestCheckTx{Tx: txBytes})
	code, data, log := res.Code, res.Data, res.Log
	if code != codeExp {
		fmt.Println("Failed test: CheckTx")
		fmt.Printf("CheckTx response code was unexpected. Got %v expected %v. Log: %v\n",
			code, codeExp, log)
		return errors.New("checkTx")
	}
	if !bytes.Equal(data, dataExp) {
		fmt.Println("Failed test: CheckTx")
		fmt.Printf("CheckTx response data was unexpected. Got %X expected %X\n",
			data, dataExp)
		return errors.New("checkTx")
	}
	fmt.Println("Passed test: CheckTx")
	return nil
}
