package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Broadcast evidence of the misbehavior.
//
// ```shell
// curl 'localhost:26657/broadcast_evidence?evidence='
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// info, err := client.BroadcastEvidence(types.DuplicateVoteEvidenc{PubKey: ev.PubKey, VoteA: *ev.VoteA, VoteB: *ev.VoteB})
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// ```
//
// | Parameter | Type           | Default | Required | Description            |
// |-----------+----------------+---------+----------+------------------------|
// | evidence  | types.Evidence | nil     | true     | Amino-encoded evidence |
func BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	err := evidencePool.AddEvidence(ev)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting evidence, adding evidence: %v", err)
	}
	return &ctypes.ResultBroadcastEvidence{Hash: ev.Hash()}, nil
}
