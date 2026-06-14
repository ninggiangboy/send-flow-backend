package observability

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

type APIKeyMetrics struct {
	authAttempts *prometheus.CounterVec
}

func NewAPIKeyMetrics(reg prometheus.Registerer) (*APIKeyMetrics, error) {
	m := &APIKeyMetrics{
		authAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_api_key_auth_attempts_total",
			Help: "Total API key auth attempts by result.",
		}, []string{"result"}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	if err := reg.Register(m.authAttempts); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			m.authAttempts = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			return nil, err
		}
	}
	return m, nil
}

func (m *APIKeyMetrics) RecordAuthAttempt(result string) {
	m.authAttempts.WithLabelValues(result).Inc()
}
