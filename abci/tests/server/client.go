package testsuite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	mrand "math/rand"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func InitChain(ctx context.Context, client abciclient.Client) error {
	total := 10
	vals := make([]types.ValidatorUpdate, total)
	for i := 0; i < total; i++ {
		pubkey := tmrand.Bytes(33)
		// nolint:gosec // G404: Use of weak random number generator
		power := mrand.Int()
		vals[i] = types.UpdateValidator(pubkey, int64(power), "")
	}
	_, err := client.InitChain(ctx, types.RequestInitChain{
		Validators: vals,
	})
	if err != nil {
		fmt.Printf("Failed test: InitChain - %v\n", err)
		return err
	}
	fmt.Println("Passed test: InitChain")
	return nil
}

func Commit(ctx context.Context, client abciclient.Client, hashExp []byte) error {
	res, err := client.Commit(ctx)
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

func DeliverTx(ctx context.Context, client abciclient.Client, txBytes []byte, codeExp uint32, dataExp []byte) error {
	res, _ := client.DeliverTx(ctx, types.RequestDeliverTx{Tx: txBytes})
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

func CheckTx(ctx context.Context, client abciclient.Client, txBytes []byte, codeExp uint32, dataExp []byte) error {
	res, _ := client.CheckTx(ctx, types.RequestCheckTx{Tx: txBytes})
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
