package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func examplePrevote() *Vote {
	return exampleVote(VoteTypePrevote)
}

func examplePrecommit() *Vote {
	return exampleVote(VoteTypePrecommit)
}

func exampleVote(t byte) *Vote {
	var stamp, err = time.Parse(TimeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	return &Vote{
		ValidatorAddress: []byte("addr"),
		ValidatorIndex:   56789,
		Height:           12345,
		Round:            2,
		Timestamp:        stamp,
		Type:             t,
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
	vote := examplePrecommit()
	signBytes := vote.SignBytes("test_chain_id")
	signStr := string(signBytes)

	expected := `{"@chain_id":"test_chain_id","@type":"vote","block_id":{"hash":"68617368","parts":{"hash":"70617274735F68617368","total":"1000000"}},"height":"12345","round":"2","timestamp":"2017-12-25T03:00:01.234Z","type":2}`
	if signStr != expected {
		// NOTE: when this fails, you probably want to fix up consensus/replay_test too
		t.Errorf("Got unexpected sign string for Vote. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestVoteString(t *testing.T) {
	tc := []struct {
		name string
		in   string
		out  string
	}{
		{"Precommit", examplePrecommit().String(), `Vote{56789:616464720000 12345/02/2(Precommit) 686173680000 <nil> @ 2017-12-25T03:00:01.234Z}`},
		{"Prevote", examplePrevote().String(), `Vote{56789:616464720000 12345/02/1(Prevote) 686173680000 <nil> @ 2017-12-25T03:00:01.234Z}`},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(st *testing.T) {
			if tt.in != tt.out {
				t.Errorf("Got unexpected string for Proposal. Expected:\n%v\nGot:\n%v", tt.in, tt.out)
			}
		})
	}
}

func TestVoteVerifySignature(t *testing.T) {
	privVal := NewMockPV()
	pubKey := privVal.GetPubKey()

	vote := examplePrecommit()
	signBytes := vote.SignBytes("test_chain_id")

	// sign it
	err := privVal.SignVote("test_chain_id", vote)
	require.NoError(t, err)

	// verify the same vote
	valid := pubKey.VerifyBytes(vote.SignBytes("test_chain_id"), vote.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	precommit := new(Vote)
	bs, err := cdc.MarshalBinary(vote)
	require.NoError(t, err)
	err = cdc.UnmarshalBinary(bs, &precommit)
	require.NoError(t, err)

	// verify the transmitted vote
	newSignBytes := precommit.SignBytes("test_chain_id")
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubKey.VerifyBytes(newSignBytes, precommit.Signature)
	require.True(t, valid)
}

func TestIsVoteTypeValid(t *testing.T) {
	tc := []struct {
		name string
		in   byte
		out  bool
	}{
		{"Prevote", VoteTypePrevote, true},
		{"Precommit", VoteTypePrecommit, true},
		{"InvalidType", byte(3), false},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(st *testing.T) {
			if rs := IsVoteTypeValid(tt.in); rs != tt.out {
				t.Errorf("Got unexpected Vote type. Expected:\n%v\nGot:\n%v", rs, tt.out)
			}
		})
	}
}
