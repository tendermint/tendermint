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
	b, err := payload.NewBytes(payload.Options{
		Size: 1024,
	})
	if err != nil {
		t.Fatalf("generating payload %s", err)
	}
	p, err := payload.FromBytes(b)
	if err != nil {
		t.Fatalf("reading payload %s", err)
	}
	if p.Size != 1024 {
		t.Fatalf("payload size value %d does not match expected %d", p.Size, 1024)
	}
}
