package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

type SyncMetrics struct {
	batchesTotal  prometheus.Counter
	rowsTotal     prometheus.Counter
	failuresTotal prometheus.Counter
	duration      prometheus.Histogram
	lastSyncTS    prometheus.Gauge
	currentLag    prometheus.Gauge
}

func NewSyncMetrics(reg prometheus.Registerer) (*SyncMetrics, error) {
	m := &SyncMetrics{
		batchesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sendflow_sync_batches_total",
			Help: "Total number of sync batches processed.",
		}),
		rowsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sendflow_sync_rows_total",
			Help: "Total number of rows synced.",
		}),
		failuresTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sendflow_sync_failures_total",
			Help: "Total number of failed sync batches.",
		}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sendflow_sync_duration_seconds",
			Help:    "Duration of sync batch operations.",
			Buckets: prometheus.DefBuckets,
		}),
		lastSyncTS: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sendflow_sync_last_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful sync.",
		}),
		currentLag: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sendflow_sync_current_lag_seconds",
			Help: "Current sync lag in seconds based on last_synced_at versus wall clock.",
		}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	for _, collector := range []prometheus.Collector{
		m.batchesTotal, m.rowsTotal, m.failuresTotal,
		m.duration, m.lastSyncTS, m.currentLag,
	} {
		if err := reg.Register(collector); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				_ = are
			} else {
				return nil, err
			}
		}
	}
	return m, nil
}

func (m *SyncMetrics) RecordBatch(rows int, duration float64, err error) {
	m.batchesTotal.Inc()
	m.rowsTotal.Add(float64(rows))
	m.duration.Observe(duration)
	if err != nil {
		m.failuresTotal.Inc()
	} else {
		m.lastSyncTS.SetToCurrentTime()
	}
}

func (m *SyncMetrics) SetLag(lagSeconds float64) {
	m.currentLag.Set(lagSeconds)
}
