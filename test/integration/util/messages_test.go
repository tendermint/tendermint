package util

import (
	"bytes"
	"testing"
	"time"

	"github.com/ds-test-framework/scheduler/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmsg "github.com/tendermint/tendermint/proto/tendermint/consensus"
	prototypes "github.com/tendermint/tendermint/proto/tendermint/types"
	ttypes "github.com/tendermint/tendermint/types"
)

func TestChangeVote(t *testing.T) {
	var stamp, err = time.Parse(time.RFC3339Nano, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	blockId := ttypes.BlockID{
		Hash: tmhash.Sum([]byte("blockID_hash")),
		PartSetHeader: ttypes.PartSetHeader{
			Total: 100000,
			Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
		},
	}

	vote := &prototypes.Vote{
		Type:             prototypes.PrevoteType,
		Height:           12345,
		Round:            2,
		Timestamp:        stamp,
		BlockID:          blockId.ToProto(),
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		ValidatorIndex:   56789,
	}

	replica := &types.Replica{
		Info: map[string]interface{}{
			"chain_id": "chain-6DYikF",
			"private_key": map[string]interface{}{
				"type": "ed25519",
				"key":  "8O8n/xnT93YsfzvTdU35Wdsoht6FlWMjIPxZplbpGgUScImqzhPZc5LCAEGC5kt9a/MyJfMLwTklv4SKMC/ORA==",
			},
		},
	}

	chainID, err := GetChainID(replica)
	if err != nil {
		t.Error(err)
	}
	privKey, err := GetPrivKey(replica)
	if err != nil {
		t.Error(err)
	}

	signBytes := ttypes.VoteSignBytes(chainID, vote)
	sig, err := privKey.Sign(signBytes)
	if err != nil {
		t.Error(err)
	}

	vote.Signature = sig

	voteMsg := &TMessageWrapper{
		Type: Prevote,
		Msg: &tmsg.Message{
			Sum: &tmsg.Message_Vote{
				Vote: &tmsg.Vote{
					Vote: vote,
				},
			},
		},
	}

	newVoteMsg, err := ChangeVoteToNil(replica, voteMsg)
	if err != nil {
		t.Error(err)
	}
	newvote := newVoteMsg.Msg.GetVote().Vote
	newBlockID, err := ttypes.BlockIDFromProto(&newvote.BlockID)
	if err != nil {
		t.Error(err)
	}

	if newBlockID.Equals(blockId) || bytes.Equal(newvote.Signature, sig) {
		t.Error("Signature or block ID did not change")
	}
	if newvote.BlockID.Hash != nil {
		t.Error("Vote did not change to nil")
	}
}
