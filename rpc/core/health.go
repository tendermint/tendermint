package core

import (
	"time"

	"errors"
	"fmt"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Get node health. Returns HTTP `code 200` if creating blocks,
// `code 500` in other case.
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
	var latestBlockMeta *types.BlockMeta
	latestHeight := blockStore.Height()
	if latestHeight != 0 {
		latestBlockMeta = blockStore.LoadBlockMeta(latestHeight)
	}

	latestBlockTimeout := cfg.Consensus.Commit(latestBlockMeta.Header.Time).UnixNano()
	currentTime := time.Now().UnixNano()
	blockDelayTime := time.Duration(latestBlockTimeout-currentTime) * time.Nanosecond
	if blockDelayTime > 0 {
		return nil, errors.New(fmt.Sprintf("Unhealthy. Block delayed for %d sec.", blockDelayTime/time.Second))
	}

	return &ctypes.ResultHealth{}, nil
}
