package protoio_test

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func aVote(t testing.TB) *types.Vote {
	t.Helper()
	var stamp, err = time.Parse(types.TimeFormat, "2017-12-25T03:00:01.234Z")
	require.NoError(t, err)

	return &types.Vote{
		Type:      tmproto.SignedMsgType(byte(tmproto.PrevoteType)),
		Height:    12345,
		Round:     2,
		Timestamp: stamp,
		BlockID: types.BlockID{
			Hash: crypto.Checksum([]byte("blockID_hash")),
			PartSetHeader: types.PartSetHeader{
				Total: 1000000,
				Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
			},
		},
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		ValidatorIndex:   56789,
	}
}

type excludedMarshalTo struct {
	msg proto.Message
}

func (emt *excludedMarshalTo) ProtoMessage() {}
func (emt *excludedMarshalTo) String() string {
	return emt.msg.String()
}
func (emt *excludedMarshalTo) Reset() {
	emt.msg.Reset()
}
func (emt *excludedMarshalTo) Marshal() ([]byte, error) {
	return proto.Marshal(emt.msg)
}

var _ proto.Message = (*excludedMarshalTo)(nil)

var sink interface{}

func BenchmarkMarshalDelimitedWithMarshalTo(b *testing.B) {
	msgs := []proto.Message{
		aVote(b).ToProto(),
	}
	benchmarkMarshalDelimited(b, msgs)
}

func BenchmarkMarshalDelimitedNoMarshalTo(b *testing.B) {
	msgs := []proto.Message{
		&excludedMarshalTo{aVote(b).ToProto()},
	}
	benchmarkMarshalDelimited(b, msgs)
}

func benchmarkMarshalDelimited(b *testing.B, msgs []proto.Message) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, msg := range msgs {
			blob, err := protoio.MarshalDelimited(msg)
			require.Nil(b, err)
			sink = blob
		}
	}

	if sink == nil {
		b.Fatal("Benchmark did not run")
	}

	// Reset the sink.
	sink = (interface{})(nil)
}
