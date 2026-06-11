package observability

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	requests *prometheus.CounterVec
	latency  *prometheus.HistogramVec
}

func NewHTTPMetrics(reg prometheus.Registerer) (*HTTPMetrics, error) {
	m := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_http_requests_total",
			Help: "Total HTTP requests handled by send-flow.",
		}, []string{"method", "route", "status"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sendflow_http_request_duration_seconds",
			Help:    "HTTP request duration by route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	if err := reg.Register(m.requests); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			m.requests = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			return nil, err
		}
	}
	if err := reg.Register(m.latency); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			m.latency = are.ExistingCollector.(*prometheus.HistogramVec)
		} else {
			return nil, err
		}
	}
	return m, nil
}

func (m *HTTPMetrics) Record(method, route, status string, duration float64) {
	m.requests.WithLabelValues(method, route, status).Inc()
	m.latency.WithLabelValues(method, route, status).Observe(duration)
}

func (m *HTTPMetrics) Middleware(route string, log *slog.Logger) func(http.Handler) http.Handler {
	if route == "" {
		route = "unknown"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &ResponseRecorder{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(rec, r)

			status := strconv.Itoa(rec.Status)
			m.requests.WithLabelValues(r.Method, route, status).Inc()
			m.latency.WithLabelValues(r.Method, route, status).Observe(time.Since(start).Seconds())
			if log != nil {
				log.Info("http request", "method", r.Method, "route", route, "status", rec.Status, "duration_ms", time.Since(start).Milliseconds())
			}
		})
	}
}

type ResponseRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *ResponseRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *ResponseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *ResponseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
