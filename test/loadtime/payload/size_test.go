package payload_test

import (
	"testing"

	"github.com/tendermint/tendermint/test/loadtime/payload"
)

const payloadSizeTarget = 1024 // 1kb

func TestCalculateSize(t *testing.T) {
	s, err := payload.CalculateUnpaddedSizeBytes()
	if err != nil {
		t.Fatalf("calculating unpadded size %s", err)
	}
	if s > payloadSizeTarget {
		t.Fatalf("unpadded payload size %d exceeds target %d", s, payloadSizeTarget)
	}
}
