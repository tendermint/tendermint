package consensus_test

import (
	"encoding/hex"
	math "math"
	"testing"

	proto "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)


func TestHasVoteVector(t *testing.T) {
	testcases := []struct{
		hasVote tmcons.HasVote
		expBytes string
		Err bool
	}{
		{tmcons.HasVote{1,3,tmproto.PrevoteType,1},"3a080801100318012001", false},
		{tmcons.HasVote{2,2,tmproto.PrecommitType,2},"3a080802100218022002", false},
		{tmcons.HasVote{math.MaxInt64,math.MaxInt32,tmproto.ProposalType,math.MaxInt32},"3a1808ffffffffffffffff7f10ffffffff07182020ffffffff07", false},
	}

	for i, tc :=range  testcases {
		msg := tmcons.Message{&tmcons.Message_HasVote{HasVote: &tc.hasVote}}
		bz, err := proto.Marshal(&msg)
		assert.NoError(t, err)
		assert.Equal(t, tc.expBytes, hex.EncodeToString(bz), "test vector failed", i)
	}
}
