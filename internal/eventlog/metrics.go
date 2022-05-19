package eventlog

import "github.com/prometheus/client_golang/prometheus"

const MetricsSubsystem = "eventlog"

//go:generate go run ../../scripts/metricsgen -struct=Metrics

// Metrics define the metrics exported by the eventlog package.
type Metrics struct {

	// Number of items currently resident in the event log.
	numItems prometheus.GaugeVec
}
