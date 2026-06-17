package observability

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

type RedisMetrics struct {
	cacheHits     *prometheus.CounterVec
	cacheMisses   *prometheus.CounterVec
	loadSuccesses *prometheus.CounterVec
	loadErrors    *prometheus.CounterVec
	setErrors     *prometheus.CounterVec
	deleteErrors  *prometheus.CounterVec
	lockAcquired  *prometheus.CounterVec
	lockContended *prometheus.CounterVec
	lockErrors    *prometheus.CounterVec
	loadDuration  *prometheus.HistogramVec
	lockDuration  *prometheus.HistogramVec
}

func NewRedisMetrics(reg prometheus.Registerer) (*RedisMetrics, error) {
	m := &RedisMetrics{
		cacheHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_cache_hits_total",
			Help: "Total Redis cache hits by module and resource.",
		}, []string{"module", "resource"}),
		cacheMisses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_cache_misses_total",
			Help: "Total Redis cache misses by module and resource.",
		}, []string{"module", "resource"}),
		loadSuccesses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_cache_load_successes_total",
			Help: "Total successful cache-aside loads from source by module and resource.",
		}, []string{"module", "resource"}),
		loadErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_cache_load_errors_total",
			Help: "Total cache-aside load errors from source by module and resource.",
		}, []string{"module", "resource"}),
		setErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_cache_set_errors_total",
			Help: "Total Redis cache set errors by module and resource.",
		}, []string{"module", "resource"}),
		deleteErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_cache_delete_errors_total",
			Help: "Total Redis cache delete errors by module and resource.",
		}, []string{"module", "resource"}),
		lockAcquired: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_lock_acquired_total",
			Help: "Total Redis locks acquired by module and resource.",
		}, []string{"module", "resource"}),
		lockContended: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_lock_contended_total",
			Help: "Total Redis lock contention events by module and resource.",
		}, []string{"module", "resource"}),
		lockErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_redis_lock_errors_total",
			Help: "Total Redis lock errors by module and resource.",
		}, []string{"module", "resource"}),
		loadDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sendflow_redis_cache_load_duration_seconds",
			Help:    "Duration of cache-aside source loads by module and resource.",
			Buckets: prometheus.DefBuckets,
		}, []string{"module", "resource"}),
		lockDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sendflow_redis_lock_duration_seconds",
			Help:    "Duration of Redis lock hold time by module and resource.",
			Buckets: prometheus.DefBuckets,
		}, []string{"module", "resource"}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	for _, c := range []prometheus.Collector{
		m.cacheHits, m.cacheMisses, m.loadSuccesses, m.loadErrors,
		m.setErrors, m.deleteErrors,
		m.lockAcquired, m.lockContended, m.lockErrors,
		m.loadDuration, m.lockDuration,
	} {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				continue
			}
			return nil, err
		}
	}
	return m, nil
}

func (m *RedisMetrics) RecordCacheHit(module, resource string) {
	m.cacheHits.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordCacheMiss(module, resource string) {
	m.cacheMisses.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordCacheLoadSuccess(module, resource string) {
	m.loadSuccesses.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordCacheLoadError(module, resource string) {
	m.loadErrors.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordCacheSetError(module, resource string) {
	m.setErrors.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordCacheDeleteError(module, resource string) {
	m.deleteErrors.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordLockAcquired(module, resource string) {
	m.lockAcquired.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordLockContended(module, resource string) {
	m.lockContended.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) RecordLockError(module, resource string) {
	m.lockErrors.WithLabelValues(module, resource).Inc()
}

func (m *RedisMetrics) ObserveLoadDuration(module, resource string, seconds float64) {
	m.loadDuration.WithLabelValues(module, resource).Observe(seconds)
}

func (m *RedisMetrics) ObserveLockDuration(module, resource string, seconds float64) {
	m.lockDuration.WithLabelValues(module, resource).Observe(seconds)
}
