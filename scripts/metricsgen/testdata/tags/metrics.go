package tags

import "github.com/go-kit/kit/metrics"

//go:generate go run ../../../../scripts/metricsgen -struct=Metrics

type Metrics struct {
	WithLabels     metrics.Counter   `metricsgen_labels:"step,time"`
	WithExpBuckets metrics.Histogram `metricsgen_bucketsType:"exp" metricsgen_bucketSizes:".1,100,8"`
	WithBuckets    metrics.Histogram `metricsgen_bucketSizes:"1, 2, 3, 4, 5"`
	Named          metrics.Counter   `metricsgen_name:"metric_with_name"`
}
