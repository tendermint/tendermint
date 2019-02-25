package core

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// Returns right away, with no response. Does not wait for CheckTx nor
// DeliverTx results.
//
// ```shell
// curl 'localhost:26657/broadcast_tx_async?tx="123"'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.BroadcastTxAsync("123")
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"hash": "E39AAB7A537ABAA237831742DCE1117F187C3C52",
// 		"log": "",
// 		"data": "",
// 		"code": "0"
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description     |
// |-----------+------+---------+----------+-----------------|
// | tx        | Tx   | nil     | true     | The transaction |
//
// To send a UTF8 string byte array, you simply quote the value of the tx parameter on the GET request:
//
//     GET http://myserver:46657/broadcast_tx_async?tx="hello"
//
// This sends a 5 byte transaction "h e l l o" [68 65 6c 6c 6f]
//
//     GET http://myserver:46657/broadcast_tx_async?tx="5"
//
// This sends a 4 byte transaction "5" (UTF8) [e2 82 ac 35]
//
// To send as raw hex, you MUST omit the quotes AND prefix the hex string with 0x
//
//     GET http://myserver:46657/broadcast_tx_async?tx=0x01020304
//
// This sends a 4 byte transaction [01 02 03 04]
//
// If you prefer to POST your data as json, you have to Base64 encode your transaction byte array.
//
// ```
// POST http://myserver:46657
// {
//   "method": "broadcast_tx_async",
//   "jsonrpc": "2.0",
//   "params": {"tx": "AQIDBA=="},
//   "id": ""
// }
// ```
//
// This sends the same 4 byte transaction [01 02 03 04]
//
// Note that you CANNOT POST your transaction as hex like so:
//
// ```
// {
//   "method": "broadcast_tx_async",
//   "jsonrpc": "2.0",
//   "params": {"tx": "0x01020304"},
//   "id": ""
// }
// ```
//
// You'll get a 400 Bad Request with error: "Error converting json params to
// arguments: illegal base64 data at input byte 8"
func BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempool.CheckTx(tx, nil)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// Returns with the response from CheckTx. Does not wait for DeliverTx result.
//
// ```shell
// curl 'localhost:26657/broadcast_tx_sync?tx="456"'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.BroadcastTxSync("456")
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"jsonrpc": "2.0",
// 	"id": "",
// 	"result": {
// 		"code": "0",
// 		"data": "",
// 		"log": "",
// 		"hash": "0D33F2F03A5234F38706E43004489E061AC40A2E"
// 	},
// 	"error": ""
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description     |
// |-----------+------+---------+----------+-----------------|
// | tx        | Tx   | nil     | true     | The transaction |
//
// To send a UTF8 string byte array, you simply quote the value of the tx parameter on the GET request:
//
//     GET http://myserver:46657/broadcast_tx_sync?tx="hello"
//
// This sends a 5 byte transaction "h e l l o" [68 65 6c 6c 6f]
//
//     GET http://myserver:46657/broadcast_tx_sync?tx="5"
//
// This sends a 4 byte transaction "5" (UTF8) [e2 82 ac 35]
//
// To send as raw hex, you MUST omit the quotes AND prefix the hex string with 0x
//
//     GET http://myserver:46657/broadcast_tx_sync?tx=0x01020304
//
// This sends a 4 byte transaction [01 02 03 04]
//
// If you prefer to POST your data as json, you have to Base64 encode your transaction byte array.
//
// ```
// POST http://myserver:46657
// {
//   "method": "broadcast_tx_sync",
//   "jsonrpc": "2.0",
//   "params": {"tx": "AQIDBA=="},
//   "id": ""
// }
// ```
//
// This sends the same 4 byte transaction [01 02 03 04]
//
// Note that you CANNOT POST your transaction as hex like so:
//
// ```
// {
//   "method": "broadcast_tx_sync",
//   "jsonrpc": "2.0",
//   "params": {"tx": "0x01020304"},
//   "id": ""
// }
// ```
//
// You'll get a 400 Bad Request with error: "Error converting json params to
// arguments: illegal base64 data at input byte 8"
func BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.Response, 1)
	err := mempool.CheckTx(tx, func(res *abci.Response) {
		resCh <- res
	})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
		Hash: tx.Hash(),
	}, nil
}

// Returns with the responses from CheckTx and DeliverTx.
//
// CONTRACT: only returns error if mempool.CheckTx() errs or if we timeout
// waiting for tx to commit.
//
// If CheckTx or DeliverTx fail, no error will be returned, but the returned result
// will contain a non-OK ABCI code.
//
// ```shell
// curl 'localhost:26657/broadcast_tx_commit?tx="789"'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.BroadcastTxCommit("789")
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"height": "26682",
// 		"hash": "75CA0F856A4DA078FC4911580360E70CEFB2EBEE",
// 		"deliver_tx": {
// 			"log": "",
// 			"data": "",
// 			"code": "0"
// 		},
// 		"check_tx": {
// 			"log": "",
// 			"data": "",
// 			"code": "0"
// 		}
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description     |
// |-----------+------+---------+----------+-----------------|
// | tx        | Tx   | nil     | true     | The transaction |
//
// To send a UTF8 string byte array, you simply quote the value of the tx parameter on the GET request:
//
//     GET http://myserver:46657/broadcast_tx_commit?tx="hello"
//
// This sends a 5 byte transaction "h e l l o" [68 65 6c 6c 6f]
//
//     GET http://myserver:46657/broadcast_tx_commit?tx="5"
//
// This sends a 4 byte transaction "5" (UTF8) [e2 82 ac 35]
//
// To send as raw hex, you MUST omit the quotes AND prefix the hex string with 0x
//
//     GET http://myserver:46657/broadcast_tx_commit?tx=0x01020304
//
// This sends a 4 byte transaction [01 02 03 04]
//
// If you prefer to POST your data as json, you have to Base64 encode your transaction byte array.
//
// ```
// POST http://myserver:46657
// {
//   "method": "broadcast_tx_commit",
//   "jsonrpc": "2.0",
//   "params": {"tx": "AQIDBA=="},
//   "id": ""
// }
// ```
//
// This sends the same 4 byte transaction [01 02 03 04]
//
// Note that you CANNOT POST your transaction as hex like so:
//
// ```
// {
//   "method": "broadcast_tx_commit",
//   "jsonrpc": "2.0",
//   "params": {"tx": "0x01020304"},
//   "id": ""
// }
// ```
//
// You'll get a 400 Bad Request with error: "Error converting json params to
// arguments: illegal base64 data at input byte 8"
func BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	// Subscribe to tx being committed in block.
	ctx, cancel := context.WithTimeout(context.Background(), subscribeTimeout)
	defer cancel()
	q := types.EventQueryTxFor(tx)
	deliverTxSub, err := eventBus.Subscribe(ctx, "mempool", q)
	if err != nil {
		err = errors.Wrap(err, "failed to subscribe to tx")
		logger.Error("Error on broadcast_tx_commit", "err", err)
		return nil, err
	}
	defer eventBus.Unsubscribe(context.Background(), "mempool", q)

	// Broadcast tx and wait for CheckTx result
	checkTxResCh := make(chan *abci.Response, 1)
	err = mempool.CheckTx(tx, func(res *abci.Response) {
		checkTxResCh <- res
	})
	if err != nil {
		logger.Error("Error on broadcastTxCommit", "err", err)
		return nil, fmt.Errorf("Error on broadcastTxCommit: %v", err)
	}
	checkTxResMsg := <-checkTxResCh
	checkTxRes := checkTxResMsg.GetCheckTx()
	if checkTxRes.Code != abci.CodeTypeOK {
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, nil
	}

	// Wait for the tx to be included in a block or timeout.
	// TODO: configurable?
	var deliverTxTimeout = rpcserver.WriteTimeout / 2
	select {
	case msg := <-deliverTxSub.Out(): // The tx was included in a block.
		deliverTxRes := msg.Data().(types.EventDataTx)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: deliverTxRes.Result,
			Hash:      tx.Hash(),
			Height:    deliverTxRes.Height,
		}, nil
	case <-deliverTxSub.Cancelled():
		err = errors.New("deliverTxSub was cancelled. Did the Tendermint stop?")
		logger.Error("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	case <-time.After(deliverTxTimeout):
		err = errors.New("Timed out waiting for tx to be included in a block")
		logger.Error("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	}
}

// Get unconfirmed transactions (maximum ?limit entries) including their number.
//
// ```shell
// curl 'localhost:26657/unconfirmed_txs'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.UnconfirmedTxs()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "result" : {
//       "txs" : [],
//       "total_bytes" : "0",
//       "n_txs" : "0",
//       "total" : "0"
//     },
//     "jsonrpc" : "2.0",
//     "id" : ""
//   }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description                          |
// |-----------+------+---------+----------+--------------------------------------|
// | limit     | int  | 30      | false    | Maximum number of entries (max: 100) |
func UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	// reuse per_page validator
	limit = validatePerPage(limit)

	txs := mempool.ReapMaxTxs(limit)
	return &ctypes.ResultUnconfirmedTxs{
		Count:      len(txs),
		Total:      mempool.Size(),
		TotalBytes: mempool.TxsBytes(),
		Txs:        txs}, nil
}

// Get number of unconfirmed transactions.
//
// ```shell
// curl 'localhost:26657/num_unconfirmed_txs'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.UnconfirmedTxs()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "jsonrpc" : "2.0",
//   "id" : "",
//   "result" : {
//     "n_txs" : "0",
//     "total_bytes" : "0",
//     "txs" : null,
//     "total" : "0"
//   }
// }
// ```
func NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{
		Count:      mempool.Size(),
		Total:      mempool.Size(),
		TotalBytes: mempool.TxsBytes()}, nil
}
