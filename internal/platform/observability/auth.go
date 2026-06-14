package observability

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

type AuthMetrics struct {
	authAttempts     *prometheus.CounterVec
	loginLatency     *prometheus.HistogramVec
	activeSessions   prometheus.Gauge
	sessionCreations prometheus.Counter
}

func NewAuthMetrics(reg prometheus.Registerer) (*AuthMetrics, error) {
	m := &AuthMetrics{
		authAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_auth_attempts_total",
			Help: "Total auth attempts by method and result.",
		}, []string{"method", "result"}),
		loginLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sendflow_login_duration_seconds",
			Help:    "Login latency by method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"}),
		activeSessions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sendflow_active_sessions",
			Help: "Current number of active sessions.",
		}),
		sessionCreations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sendflow_session_creations_total",
			Help: "Total session creations.",
		}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	for _, c := range []prometheus.Collector{m.authAttempts, m.loginLatency, m.activeSessions, m.sessionCreations} {
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

func (m *AuthMetrics) RecordAuthAttempt(method, result string) {
	m.authAttempts.WithLabelValues(method, result).Inc()
}

func (m *AuthMetrics) RecordLoginDuration(method string, seconds float64) {
	m.loginLatency.WithLabelValues(method).Observe(seconds)
}

func (m *AuthMetrics) SetActiveSessions(count int64) {
	m.activeSessions.Set(float64(count))
}

func (m *AuthMetrics) IncSessionCreations() {
	m.sessionCreations.Inc()
}
