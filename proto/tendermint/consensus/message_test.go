package consensus_test

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestHasVoteVector(t *testing.T) {
	testCases := []struct {
		msg      tmcons.HasVote
		expBytes string
	}{
		{tmcons.HasVote{1, 3, tmproto.PrevoteType, 1}, "3a080801100318012001"},
		{tmcons.HasVote{2, 2, tmproto.PrecommitType, 2}, "3a080802100218022002"},
		{tmcons.HasVote{math.MaxInt64, math.MaxInt32, tmproto.ProposalType, math.MaxInt32},
			"3a1808ffffffffffffffff7f10ffffffff07182020ffffffff07"},
	}

	for i, tc := range testCases {
		msg := tmcons.Message{&tmcons.Message_HasVote{HasVote: &tc.msg}}
		bz, err := proto.Marshal(&msg)
		require.NoError(t, err)
		require.Equal(t, tc.expBytes, hex.EncodeToString(bz), "test vector failed", i)
	}
}
