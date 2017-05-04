package testsuite

import (
	"bytes"
	"errors"
	"fmt"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

func InitChain(client abcicli.Client) error {
	total := 10
	vals := make([]*types.Validator, total)
	for i := 0; i < total; i++ {
		pubkey := crypto.GenPrivKeyEd25519FromSecret([]byte(cmn.Fmt("test%d", i))).PubKey().Bytes()
		power := cmn.RandInt()
		vals[i] = &types.Validator{pubkey, uint64(power)}
	}
	err := client.InitChainSync(vals)
	if err != nil {
		fmt.Println("Failed test: InitChain - %v", err)
		return err
	}
	fmt.Println("Passed test: InitChain")
	return nil
}

func SetOption(client abcicli.Client, key, value string) error {
	res := client.SetOptionSync(key, value)
	_, _, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		fmt.Println("Failed test: SetOption")
		fmt.Printf("setting %v=%v: \nlog: %v", key, value, log)
		fmt.Println("Failed test: SetOption")
		return errors.New(res.Error())
	}
	fmt.Println("Passed test: SetOption")
	return nil
}

func Commit(client abcicli.Client, hashExp []byte) error {
	res := client.CommitSync()
	_, data, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		fmt.Println("Failed test: Commit")
		fmt.Printf("committing %v\nlog: %v", log)
		return errors.New(res.Error())
	}
	if !bytes.Equal(res.Data, hashExp) {
		fmt.Println("Failed test: Commit")
		fmt.Printf("Commit hash was unexpected. Got %X expected %X",
			data, hashExp)
		return errors.New("CommitTx failed")
	}
	fmt.Println("Passed test: Commit")
	return nil
}

func DeliverTx(client abcicli.Client, txBytes []byte, codeExp types.CodeType, dataExp []byte) error {
	res := client.DeliverTxSync(txBytes)
	code, data, log := res.Code, res.Data, res.Log
	if code != codeExp {
		fmt.Println("Failed test: DeliverTx")
		fmt.Printf("DeliverTx response code was unexpected. Got %v expected %v. Log: %v",
			code, codeExp, log)
		return errors.New("DeliverTx error")
	}
	if !bytes.Equal(data, dataExp) {
		fmt.Println("Failed test: DeliverTx")
		fmt.Printf("DeliverTx response data was unexpected. Got %X expected %X",
			data, dataExp)
		return errors.New("DeliverTx error")
	}
	fmt.Println("Passed test: DeliverTx")
	return nil
}

func CheckTx(client abcicli.Client, txBytes []byte, codeExp types.CodeType, dataExp []byte) error {
	res := client.CheckTxSync(txBytes)
	code, data, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		fmt.Println("Failed test: CheckTx")
		fmt.Printf("checking tx %X: %v\nlog: %v", txBytes, log)
		return errors.New(res.Error())
	}
	if code != codeExp {
		fmt.Println("Failed test: CheckTx")
		fmt.Printf("CheckTx response code was unexpected. Got %v expected %v. Log: %v",
			code, codeExp, log)
		return errors.New("CheckTx")
	}
	if !bytes.Equal(data, dataExp) {
		fmt.Println("Failed test: CheckTx")
		fmt.Printf("CheckTx response data was unexpected. Got %X expected %X",
			data, dataExp)
		return errors.New("CheckTx")
	}
	fmt.Println("Passed test: CheckTx")
	return nil
}
