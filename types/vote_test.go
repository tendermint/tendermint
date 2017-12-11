package types

import (
	"testing"
	"time"
)

func TestVoteSignable(t *testing.T) {
	var stamp, err = time.Parse(timeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		t.Fatal(err)
	}

	vote := &Vote{
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
	signBytes := SignBytes("test_chain_id", vote)
	signStr := string(signBytes)

	expected := `{"chain_id":"test_chain_id","vote":{"block_id":{"hash":"68617368","parts":{"hash":"70617274735F68617368","total":1000000}},"height":12345,"round":23456,"timestamp":"2017-12-25T03:00:01.234Z","type":2}}`
	if signStr != expected {
		// NOTE: when this fails, you probably want to fix up consensus/replay_test too
		t.Errorf("Got unexpected sign string for Vote. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}
