package observability

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

type SenderMetrics struct {
	verificationAttempts *prometheus.CounterVec
	pendingDomains       prometheus.Gauge
}

func NewSenderMetrics(reg prometheus.Registerer) (*SenderMetrics, error) {
	m := &SenderMetrics{
		verificationAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_sender_verification_attempts_total",
			Help: "Total sender domain verification attempts by result.",
		}, []string{"result"}),
		pendingDomains: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sendflow_sender_pending_domains",
			Help: "Current number of domains pending verification.",
		}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	for _, c := range []prometheus.Collector{m.verificationAttempts, m.pendingDomains} {
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

func (m *SenderMetrics) RecordVerificationAttempt(result string) {
	m.verificationAttempts.WithLabelValues(result).Inc()
}

func (m *SenderMetrics) SetPendingDomains(count int64) {
	m.pendingDomains.Set(float64(count))
}
