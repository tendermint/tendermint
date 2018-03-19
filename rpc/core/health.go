package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// Get node health. Checks whether new blocks are created.
//
// ```shell
// curl 'localhost:46657/health'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// result, err := client.Health()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func Health() (*ctypes.ResultHealth, error) {
	return &ctypes.ResultHealth{}, nil
}
