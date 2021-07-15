package main

import (
	"bytes"
	"context"
	"fmt"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

var ctx = context.Background()

func startClient(abciType string) abcicli.Client {
	// Start client
	client, err := abcicli.NewClient("tcp://127.0.0.1:26658", abciType, true)
	if err != nil {
		panic(err.Error())
	}
	logger := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	client.SetLogger(logger.With("module", "abcicli"))
	if err := client.Start(); err != nil {
		panicf("connecting to abci_app: %v", err.Error())
	}

	return client
}

func commit(client abcicli.Client, hashExp []byte) {
	res, err := client.CommitSync(ctx)
	if err != nil {
		panicf("client error: %v", err)
	}
	if !bytes.Equal(res.Data, hashExp) {
		panicf("Commit hash was unexpected. Got %X expected %X", res.Data, hashExp)
	}
}

type tx struct {
	Data    []byte
	CodeExp uint32
	DataExp []byte
}

func finalizeBlock(client abcicli.Client, txs []tx) {
	var txsData = make([][]byte, len(txs))
	for i, tx := range txs {
		txsData[i] = tx.Data
	}

	res, err := client.FinalizeBlockSync(ctx, types.RequestFinalizeBlock{Txs: txsData})
	if err != nil {
		panicf("client error: %v", err)
	}
	for i, tx := range res.Txs {
		if tx.Code != txs[i].CodeExp {
			panicf("DeliverTx response code was unexpected. Got %v expected %v. Log: %v", tx.Code, txs[i].CodeExp, tx.Log)
		}
		if !bytes.Equal(tx.Data, txs[i].DataExp) {
			panicf("DeliverTx response data was unexpected. Got %X expected %X", tx.Data, txs[i].DataExp)
		}
	}
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}
