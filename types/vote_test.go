package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	vote1 = Vote{
		ValidatorAddress: []byte("addr"),
		ValidatorIndex:   56789,
		Height:           12345,
		Round:            23456,
		Type:             byte(2),
		BlockID: BlockID{
			Hash: []byte("hash"),
			PartsHeader: PartSetHeader{
				Total: 1000000,
				Hash:  []byte("parts_hash"),
			},
		},
	}
)

func TestVoteSignable(t *testing.T) {
	signBytes := SignBytes("test_chain_id", &vote1)
	signStr := string(signBytes)

	expected := `{"chain_id":"test_chain_id","vote":{"block_id":{"hash":"68617368","parts":{"hash":"70617274735F68617368","total":1000000}},"height":12345,"round":23456,"type":2}}`
	if signStr != expected {
		// NOTE: when this fails, you probably want to fix up consensus/replay_test too
		t.Errorf("Got unexpected sign string for Vote. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestVoteCopy(t *testing.T) {
	require.Nil(t, ((*Vote)(nil)).Copy(), "Copy of a nil vote should return nil too")

	v1 := vote1.Copy()

	aTendermint := []byte("aTendermint")
	aFoo := []byte("aFoo")
	v1.ValidatorAddress = aTendermint[:]
	v2 := v1.Copy()
	require.Equal(t, v2, v1, "expecting a copy")

	copy(v2.ValidatorAddress, aFoo)
	require.NotEqual(t, v2, v1, "a mutation of v2 shouldn't mutate v1")
	require.Equal(t, string(v1.ValidatorAddress), "aTendermint", "v1.ValidatorAddress shouldn't have changed")
}
