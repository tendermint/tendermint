package payload_test

import (
	"testing"

	"github.com/tendermint/tendermint/test/loadtime/payload"
)

const payloadSizeTarget = 1024 // 1kb

func TestSize(t *testing.T) {
	s := payload.UnpaddedSizeBytes()
	if s > payloadSizeTarget {
		t.Fatalf("unpadded payload size %d exceeds target %d", s, payloadSizeTarget)
	}
}

func TestRoundTrip(t *testing.T) {
	const (
		testConns = 512
		testRate  = 4
	)
	b, err := payload.NewBytes(payload.Options{
		Size:  payloadSizeTarget,
		Conns: testConns,
		Rate:  testRate,
	})
	if err != nil {
		t.Fatalf("generating payload %s", err)
	}
	if len(b) < payloadSizeTarget {
		t.Fatalf("payload size %d less than expected %d", len(b), payloadSizeTarget)
	}
	p, err := payload.FromBytes(b)
	if err != nil {
		t.Fatalf("reading payload %s", err)
	}
	if p.Size != payloadSizeTarget {
		t.Fatalf("payload size value %d does not match expected %d", p.Size, payloadSizeTarget)
	}
	if p.Connections != testConns {
		t.Fatalf("payload size value %d does not match expected %d", p.Size, payloadSizeTarget)
	}
	if p.Size != payloadSizeTarget {
		t.Fatalf("payload size value %d does not match expected %d", p.Size, payloadSizeTarget)
	}
}
