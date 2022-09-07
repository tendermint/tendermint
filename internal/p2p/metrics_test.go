package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/proto/tendermint/p2p"
)

func TestValueToMetricsLabel(t *testing.T) {
	lc := newMetricsLabelCache()
	r := &p2p.PexResponse{}
	str := lc.ValueToMetricLabel(r)
	assert.Equal(t, "p2p_PexResponse", str)

	// subsequent calls to the function should produce the same result
	str = lc.ValueToMetricLabel(r)
	assert.Equal(t, "p2p_PexResponse", str)
}
