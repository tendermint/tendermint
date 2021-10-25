package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/proto/tendermint/statesync"
)

func TestValueToMetricsLabel(t *testing.T) {
	m := NopMetrics()
	r := &statesync.ParamsRequest{}
	str := m.ValueToMetricLabel(r)
	assert.Equal(t, "statesync_ParamsRequest", str)

	// subsequent calls to the function should produce the same result
	str = m.ValueToMetricLabel(r)
	assert.Equal(t, "statesync_ParamsRequest", str)
}
