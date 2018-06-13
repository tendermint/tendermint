package types

import (
	"testing"
	"github.com/tendermint/tendermint/types/proto3"
	"github.com/stretchr/testify/assert"
	"github.com/golang/protobuf/proto"
	"time"
)

func TestProto3Compatibility(t *testing.T) {
	// TODO(ismail): table tests instead...
	tm, err := time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Mon Jan 2 15:04:05 -0700 MST 2006")
	assert.NoError(t, err)

	pbHeader := proto3.Header{
		ChainID: "cosmos",
		Height:150,
		Time: &proto3.Timestamp{Seconds:tm.Unix(), Nanos:int32(tm.Nanosecond())},
		NumTxs: 7,
		LastBlockID: &proto3.BlockID{
			Hash: []byte("some serious hashing"),
			PartsHeader:&proto3.PartSetHeader{
				Total: 8,
				Hash: []byte("some more serious hashing"),
			},
		},
		TotalTxs: 100,
		LastCommitHash: []byte("commit hash"),
		DataHash: []byte("data hash"),
		ValidatorsHash:[]byte("validators hash"),

	}
	// TODO(ismail): add another test where parts are missing (to see if default values are treated equiv.)
	aminoHeader := Header{
		ChainID: "cosmos",
		Height:150,
		Time: tm,
		NumTxs: 7,
		LastBlockID: BlockID{
			Hash: []byte("some serious hashing"),
			PartsHeader: PartSetHeader{
				Total: 8,
				Hash: []byte("some more serious hashing"),
			},
		},
		TotalTxs: 100,
		LastCommitHash: []byte("commit hash"),
		DataHash: []byte("data hash"),
		ValidatorsHash:[]byte("validators hash"),
	}
	ab, err := cdc.MarshalBinaryBare(aminoHeader)
	assert.NoError(t, err, "unexpected error")

	pb, err := proto.Marshal(&pbHeader)
	assert.NoError(t, err, "unexpected error")
	// This works:
	assert.Equal(t, ab, pb, "encoding doesn't match")
}
