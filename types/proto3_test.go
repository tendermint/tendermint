package types

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/types/proto3"
)

func TestProto3Compatibility(t *testing.T) {
	tm, err := time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Mon Jan 2 15:04:05 -0700 MST 2006")
	assert.NoError(t, err)
	seconds := tm.Unix()
	nanos := int32(tm.Nanosecond())
	pbHeader := proto3.Header{
		ChainID: "cosmos",
		Height:  150,
		Time:    &proto3.Timestamp{Seconds: seconds, Nanos: nanos},
		NumTxs:  7,
		LastBlockID: &proto3.BlockID{
			Hash: []byte("some serious hashing"),
			PartsHeader: &proto3.PartSetHeader{
				Total: 8,
				Hash:  []byte("some more serious hashing"),
			},
		},
		TotalTxs:       100,
		LastCommitHash: []byte("commit hash"),
		DataHash:       []byte("data hash"),
		ValidatorsHash: []byte("validators hash"),
	}
	aminoHeader := Header{
		ChainID: "cosmos",
		Height:  150,
		Time:    tm,
		NumTxs:  7,
		LastBlockID: BlockID{
			Hash: []byte("some serious hashing"),
			PartsHeader: PartSetHeader{
				Total: 8,
				Hash:  []byte("some more serious hashing"),
			},
		},
		TotalTxs:       100,
		LastCommitHash: []byte("commit hash"),
		DataHash:       []byte("data hash"),
		ValidatorsHash: []byte("validators hash"),
	}
	ab, err := cdc.MarshalBinaryBare(aminoHeader)
	assert.NoError(t, err, "unexpected error")

	pb, err := proto.Marshal(&pbHeader)
	assert.NoError(t, err, "unexpected error")
	// This works:
	assert.Equal(t, ab, pb, "encoding doesn't match")

	emptyLastBlockPb := proto3.Header{
		ChainID: "cosmos",
		Height:  150,
		Time:    &proto3.Timestamp{Seconds: seconds, Nanos: nanos},
		NumTxs:  7,
		// TODO(ismail): as Jae suggested, we'll add a flag to make this obsolete:
		LastBlockID: &proto3.BlockID{
			PartsHeader: &proto3.PartSetHeader{},
		},
		TotalTxs:       100,
		LastCommitHash: []byte("commit hash"),
		DataHash:       []byte("data hash"),
		ValidatorsHash: []byte("validators hash"),
	}
	emptyLastBlockAm := Header{
		ChainID:        "cosmos",
		Height:         150,
		Time:           tm,
		NumTxs:         7,
		TotalTxs:       100,
		LastCommitHash: []byte("commit hash"),
		DataHash:       []byte("data hash"),
		ValidatorsHash: []byte("validators hash"),
	}

	ab, err = cdc.MarshalBinaryBare(emptyLastBlockAm)
	assert.NoError(t, err, "unexpected error")

	pb, err = proto.Marshal(&emptyLastBlockPb)
	assert.NoError(t, err, "unexpected error")
	// This works:
	assert.Equal(t, ab, pb, "encoding doesn't match")

	pb, err = proto.Marshal(&proto3.Header{})
	assert.NoError(t, err, "unexpected error")
	t.Log(pb)

	pb, err = proto.Marshal(&proto3.Timestamp{})
	assert.NoError(t, err, "unexpected error")
	t.Log(pb)

}
