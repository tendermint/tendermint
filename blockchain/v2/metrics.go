package v2

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this
	// package.
	MetricsSubsystem = "blockchain"
)

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// events_in
	EventsIn metrics.Counter
	// events_in
	EventsHandled metrics.Counter
	// events_out
	EventsOut metrics.Counter
	// errors_in
	ErrorsIn metrics.Counter
	// errors_handled
	ErrorsHandled metrics.Counter
	// errors_out
	ErrorsOut metrics.Counter
	// events_shed
	EventsShed metrics.Counter
	// events_sent
	EventsSent metrics.Counter
	// errors_sent
	ErrorsSent metrics.Counter
	// errors_shed
	ErrorsShed metrics.Counter
}

// Can we burn in the routine name here?
func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics {
	labels := []string{}
	for i := 0; i < len(labelsAndValues); i += 2 {
		labels = append(labels, labelsAndValues[i])
	}
	return &Metrics{
		EventsIn: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "events_in",
			Help:      "Events read from the channel.",
		}, labels).With(labelsAndValues...),
		EventsHandled: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "events_handled",
			Help:      "Events handled",
		}, labels).With(labelsAndValues...),
		EventsOut: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "events_out",
			Help:      "Events output from routine.",
		}, labels).With(labelsAndValues...),
		ErrorsIn: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "errors_in",
			Help:      "Errors read from the channel.",
		}, labels).With(labelsAndValues...),
		ErrorsHandled: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "errors_handled",
			Help:      "Errors handled.",
		}, labels).With(labelsAndValues...),
		ErrorsOut: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "errors_out",
			Help:      "Errors output from routine.",
		}, labels).With(labelsAndValues...),
		ErrorsSent: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "errors_sent",
			Help:      "Errors sent to routine.",
		}, labels).With(labelsAndValues...),
		ErrorsShed: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "errors_shed",
			Help:      "Errors dropped from sending.",
		}, labels).With(labelsAndValues...),
		EventsSent: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "events_sent",
			Help:      "Events sent to routine.",
		}, labels).With(labelsAndValues...),
		EventsShed: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "events_shed",
			Help:      "Events dropped from sending.",
		}, labels).With(labelsAndValues...),
	}
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		EventsIn:      discard.NewCounter(),
		EventsHandled: discard.NewCounter(),
		EventsOut:     discard.NewCounter(),
		ErrorsIn:      discard.NewCounter(),
		ErrorsHandled: discard.NewCounter(),
		ErrorsOut:     discard.NewCounter(),
		EventsShed:    discard.NewCounter(),
		EventsSent:    discard.NewCounter(),
		ErrorsSent:    discard.NewCounter(),
		ErrorsShed:    discard.NewCounter(),
	}
}
