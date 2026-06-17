package observability

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type AttachmentMetrics struct {
	uploadSuccess   *prometheus.CounterVec
	uploadFailure   *prometheus.CounterVec
	bytesUploaded   prometheus.Counter
	uploadDuration  prometheus.Histogram
	attachmentCount prometheus.Histogram
}

func NewAttachmentMetrics(reg prometheus.Registerer) (*AttachmentMetrics, error) {
	m := &AttachmentMetrics{
		uploadSuccess: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_attachment_uploads_total",
			Help: "Total attachment uploads by result.",
		}, []string{"result"}),
		uploadFailure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sendflow_attachment_upload_errors_total",
			Help: "Total attachment upload errors by result.",
		}, []string{"result"}),
		bytesUploaded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sendflow_attachment_upload_bytes_total",
			Help: "Total bytes uploaded for attachments.",
		}),
		uploadDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sendflow_attachment_upload_duration_seconds",
			Help:    "Attachment upload latency in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		}),
		attachmentCount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sendflow_attachment_count_per_request",
			Help:    "Number of attachments per send request.",
			Buckets: []float64{1, 2, 3, 5, 10},
		}),
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	for _, c := range []prometheus.Collector{m.uploadSuccess, m.uploadFailure, m.bytesUploaded, m.uploadDuration, m.attachmentCount} {
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

func (m *AttachmentMetrics) RecordUploadSuccess() {
	m.uploadSuccess.WithLabelValues("success").Inc()
}

func (m *AttachmentMetrics) RecordUploadFailure() {
	m.uploadFailure.WithLabelValues("failure").Inc()
}

func (m *AttachmentMetrics) AddBytesUploaded(bytes int64) {
	m.bytesUploaded.Add(float64(bytes))
}

func (m *AttachmentMetrics) ObserveUploadDuration(d time.Duration) {
	m.uploadDuration.Observe(d.Seconds())
}

func (m *AttachmentMetrics) ObserveAttachmentCount(count int) {
	m.attachmentCount.Observe(float64(count))
}
