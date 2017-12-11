package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wire "github.com/tendermint/go-wire"
)

func exampleVote() *Vote {
	var stamp, err = time.Parse(timeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	return &Vote{
		ValidatorAddress: []byte("addr"),
		ValidatorIndex:   56789,
		Height:           12345,
		Round:            23456,
		Timestamp:        stamp,
		Type:             byte(2),
		BlockID: BlockID{
			Hash: []byte("hash"),
			PartsHeader: PartSetHeader{
				Total: 1000000,
				Hash:  []byte("parts_hash"),
			},
		},
	}
}

func TestVoteSignable(t *testing.T) {
	vote := exampleVote()
	signBytes := SignBytes("test_chain_id", vote)
	signStr := string(signBytes)

	expected := `{"chain_id":"test_chain_id","vote":{"block_id":{"hash":"68617368","parts":{"hash":"70617274735F68617368","total":1000000}},"height":12345,"round":23456,"timestamp":"2017-12-25T03:00:01.234Z","type":2}}`
	if signStr != expected {
		// NOTE: when this fails, you probably want to fix up consensus/replay_test too
		t.Errorf("Got unexpected sign string for Vote. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestVoteVerifySignature(t *testing.T) {
	privVal := GenPrivValidatorFS("")
	pubKey := privVal.GetPubKey()

	vote := exampleVote()
	signBytes := SignBytes("test_chain_id", vote)

	// sign it
	signature, err := privVal.Signer.Sign(signBytes)
	require.NoError(t, err)

	// verify the same vote
	valid := pubKey.VerifyBytes(SignBytes("test_chain_id", vote), signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	precommit := new(Vote)
	bs := wire.BinaryBytes(vote)
	err = wire.ReadBinaryBytes(bs, &precommit)
	require.NoError(t, err)

	// verify the transmitted vote
	newSignBytes := SignBytes("test_chain_id", precommit)
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubKey.VerifyBytes(newSignBytes, signature)
	require.True(t, valid)
}
