package payload

import (
	"math"

	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func CalculateUnpaddedSizeBytes() (int, error) {
	p := &Payload{
		Time:        timestamppb.Now(),
		Connections: math.MaxUint64,
		Rate:        math.MaxUint64,
		Size:        math.MaxUint64,
		Padding:     make([]byte, 1),
	}
	b, err := proto.Marshal(p)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}
