package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// Broadcast evidence of the misbehavior.
//
// ```shell
// curl 'localhost:26657/broadcast_evidence?evidence={amino-encoded DuplicateVoteEvidence}'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// res, err := client.BroadcastEvidence(&types.DuplicateVoteEvidence{PubKey: ev.PubKey, VoteA: ev.VoteA, VoteB: ev.VoteB})
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// ```
//
// | Parameter | Type           | Default | Required | Description                 |
// |-----------+----------------+---------+----------+-----------------------------|
// | evidence  | types.Evidence | nil     | true     | Amino-encoded JSON evidence |
func BroadcastEvidence(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	err := evidencePool.AddEvidence(ev)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultBroadcastEvidence{Hash: ev.Hash()}, nil
}
