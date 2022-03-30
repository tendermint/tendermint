package testsuite

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/llmq"
)

var ctx = context.Background()

func InitChain(client abcicli.Client) error {
	const total = 10
	ld, err := llmq.Generate(crypto.RandProTxHashes(total))
	if err != nil {
		return err
	}
	validatorSet, err := types.LLMQToValidatorSetProto(*ld)
	if err != nil {
		return err
	}
	_, err = client.InitChainSync(context.Background(), types.RequestInitChain{
		ValidatorSet: validatorSet,
	})
	if err != nil {
		fmt.Printf("Failed test: InitChain - %v\n", err)
		return err
	}
	fmt.Println("Passed test: InitChain")
	return nil
}

func Commit(client abcicli.Client, hashExp []byte) error {
	res, err := client.CommitSync(ctx)
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
	res, _ := client.DeliverTxSync(ctx, types.RequestDeliverTx{Tx: txBytes})
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
	res, _ := client.CheckTxSync(ctx, types.RequestCheckTx{Tx: txBytes})
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
