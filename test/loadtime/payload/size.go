package payload

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"

	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	p := &Payload{
		Time:        timestamppb.Now(),
		Connections: math.MaxUint64,
		Rate:        math.MaxUint64,
		Size:        math.MaxUint64,
		Padding:     make([]byte, 1),
	}
	b, err := proto.Marshal(p)
	if err != nil {
		panic(err)
	}
	payloadSizeBytes = len(b)
}

const keyPrefix = "a="

var payloadSizeBytes int

type Options struct {
	Conns uint64
	Rate  uint64
	Size  uint64
}

func UnpaddedSizeBytes() int {
	return payloadSizeBytes
}

func NewBytes(o Options) ([]byte, error) {
	if o.Size < uint64(UnpaddedSizeBytes()) {
		return nil, fmt.Errorf("configured size %d not large enough to fit unpadded transaction size %d", o.Size, UnpaddedSizeBytes())
	}
	p := &Payload{
		Time:        timestamppb.Now(),
		Connections: o.Conns,
		Rate:        o.Rate,
		Size:        o.Size,
		Padding:     make([]byte, o.Size-uint64(UnpaddedSizeBytes())),
	}
	_, err := rand.Read(p.Padding)
	if err != nil {
		return nil, err
	}
	b, err := proto.Marshal(p)
	if err != nil {
		return nil, err
	}

	// prepend a single key so that the kv store only ever stores a single
	// transaction instead of storing all tx and ballooning in size.
	return append([]byte(keyPrefix), b...), nil
	//	return b, nil
}

func FromBytes(b []byte) (*Payload, error) {
	p := &Payload{}
	tr := bytes.TrimPrefix(b, []byte(keyPrefix))
	err := proto.Unmarshal(tr, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}
