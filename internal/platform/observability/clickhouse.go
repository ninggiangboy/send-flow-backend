package observability

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/prometheus/client_golang/prometheus"
)

type ClickHouseMetrics struct {
	queryDuration *prometheus.HistogramVec
	queryTotal    *prometheus.CounterVec
	queryErrors   *prometheus.CounterVec
	rowsReturned  *prometheus.HistogramVec
}

func NewClickHouseMetrics(reg prometheus.Registerer) (*ClickHouseMetrics, error) {
	m := &ClickHouseMetrics{
		queryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sendflow_clickhouse_query_duration_seconds",
			Help:    "ClickHouse query duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"query_type"}),
		queryTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_clickhouse_queries_total",
			Help: "Total number of ClickHouse queries.",
		}, []string{"query_type", "status"}),
		queryErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_clickhouse_query_errors_total",
			Help: "Total number of ClickHouse query errors.",
		}, []string{"query_type"}),
		rowsReturned: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sendflow_clickhouse_rows_returned",
			Help:    "Number of rows returned by ClickHouse queries.",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000, 10000},
		}, []string{"query_type"}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	for _, collector := range []prometheus.Collector{
		m.queryDuration, m.queryTotal, m.queryErrors, m.rowsReturned,
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

func (m *ClickHouseMetrics) ObserveQuery(queryType string, duration float64, err error, rows int) {
	status := "success"
	if err != nil {
		status = "error"
		m.queryErrors.WithLabelValues(queryType).Inc()
	}
	m.queryDuration.WithLabelValues(queryType).Observe(duration)
	m.queryTotal.WithLabelValues(queryType, status).Inc()
	m.rowsReturned.WithLabelValues(queryType).Observe(float64(rows))
}

type MetricsConn struct {
	inner     driver.Conn
	metrics   *ClickHouseMetrics
	queryType string
}

func NewMetricsConn(inner driver.Conn, metrics *ClickHouseMetrics, queryType string) *MetricsConn {
	return &MetricsConn{inner: inner, metrics: metrics, queryType: queryType}
}

func (m *MetricsConn) Contributors() []string {
	return m.inner.Contributors()
}

func (m *MetricsConn) ServerVersion() (*driver.ServerVersion, error) {
	return m.inner.ServerVersion()
}

func (m *MetricsConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	start := time.Now()
	err := m.inner.Select(ctx, dest, query, args...)
	m.metrics.ObserveQuery(m.queryType, time.Since(start).Seconds(), err, 0)
	return err
}

func (m *MetricsConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	start := time.Now()
	rows, err := m.inner.Query(ctx, query, args...)
	if err == nil && rows != nil {
		rows = &metricsRows{
			inner:     rows,
			queryType: m.queryType,
			metrics:   m.metrics,
			start:     start,
		}
		return rows, nil
	}
	m.metrics.ObserveQuery(m.queryType, time.Since(start).Seconds(), err, 0)
	return rows, err
}

type metricsRows struct {
	inner     driver.Rows
	count     int
	queryType string
	metrics   *ClickHouseMetrics
	start     time.Time
	closed    bool
}

func (r *metricsRows) Next() bool {
	hasNext := r.inner.Next()
	if hasNext {
		r.count++
	}
	return hasNext
}

func (r *metricsRows) Scan(dest ...any) error {
	return r.inner.Scan(dest...)
}

func (r *metricsRows) ScanStruct(dest any) error {
	return r.inner.ScanStruct(dest)
}

func (r *metricsRows) Columns() []string {
	return r.inner.Columns()
}

func (r *metricsRows) ColumnTypes() []driver.ColumnType {
	return r.inner.ColumnTypes()
}

func (r *metricsRows) Totals(dest ...any) error {
	return r.inner.Totals(dest...)
}

func (r *metricsRows) HasData() bool {
	return r.inner.HasData()
}

func (r *metricsRows) Close() error {
	if !r.closed {
		r.closed = true
		err := r.inner.Err()
		r.metrics.ObserveQuery(r.queryType, time.Since(r.start).Seconds(), err, r.count)
	}
	return r.inner.Close()
}

func (r *metricsRows) Err() error {
	return r.inner.Err()
}

func (m *MetricsConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	start := time.Now()
	row := m.inner.QueryRow(ctx, query, args...)
	m.metrics.ObserveQuery(m.queryType, time.Since(start).Seconds(), row.Err(), 0)
	return row
}

func (m *MetricsConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	start := time.Now()
	batch, err := m.inner.PrepareBatch(ctx, query, opts...)
	m.metrics.ObserveQuery(m.queryType, time.Since(start).Seconds(), err, 0)
	return batch, err
}

func (m *MetricsConn) Exec(ctx context.Context, query string, args ...any) error {
	start := time.Now()
	err := m.inner.Exec(ctx, query, args...)
	m.metrics.ObserveQuery(m.queryType, time.Since(start).Seconds(), err, 0)
	return err
}

func (m *MetricsConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	start := time.Now()
	err := m.inner.AsyncInsert(ctx, query, wait, args...)
	m.metrics.ObserveQuery(m.queryType, time.Since(start).Seconds(), err, 0)
	return err
}

func (m *MetricsConn) Ping(ctx context.Context) error {
	return m.inner.Ping(ctx)
}

func (m *MetricsConn) Stats() driver.Stats {
	return m.inner.Stats()
}

func (m *MetricsConn) Close() error {
	return m.inner.Close()
}
