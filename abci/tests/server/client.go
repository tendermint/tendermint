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
	_, err := client.InitChain(ctx, &types.RequestInitChain{
		Validators: vals,
	})
	if err != nil {
		fmt.Printf("Failed test: InitChain - %v\n", err)
		return err
	}
	fmt.Println("Passed test: InitChain")
	return nil
}

func Commit(ctx context.Context, client abciclient.Client) error {
	_, err := client.Commit(ctx)
	if err != nil {
		fmt.Println("Failed test: Commit")
		fmt.Printf("error while committing: %v\n", err)
		return err
	}
	fmt.Println("Passed test: Commit")
	return nil
}

func FinalizeBlock(ctx context.Context, client abciclient.Client, txBytes [][]byte, codeExp []uint32, dataExp []byte, hashExp []byte) error {
	res, _ := client.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Txs: txBytes})
	appHash := res.AppHash
	for i, tx := range res.TxResults {
		code, data, log := tx.Code, tx.Data, tx.Log
		if code != codeExp[i] {
			fmt.Println("Failed test: FinalizeBlock")
			fmt.Printf("FinalizeBlock response code was unexpected. Got %v expected %v. Log: %v\n",
				code, codeExp, log)
			return errors.New("FinalizeBlock error")
		}
		if !bytes.Equal(data, dataExp) {
			fmt.Println("Failed test:  FinalizeBlock")
			fmt.Printf("FinalizeBlock response data was unexpected. Got %X expected %X\n",
				data, dataExp)
			return errors.New("FinalizeBlock  error")
		}
	}
	if !bytes.Equal(appHash, hashExp) {
		fmt.Println("Failed test: FinalizeBlock")
		fmt.Printf("Application hash was unexpected. Got %X expected %X\n", appHash, hashExp)
		return errors.New("FinalizeBlock  error")
	}
	fmt.Println("Passed test: FinalizeBlock")
	return nil
}

func CheckTx(ctx context.Context, client abciclient.Client, txBytes []byte, codeExp uint32, dataExp []byte) error {
	res, _ := client.CheckTx(ctx, &types.RequestCheckTx{Tx: txBytes})
	code, data := res.Code, res.Data
	if code != codeExp {
		fmt.Println("Failed test: CheckTx")
		fmt.Printf("CheckTx response code was unexpected. Got %v expected %v.,",
			code, codeExp)
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
