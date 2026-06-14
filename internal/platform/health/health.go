package health

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/buildinfo"
)

type Checker func(context.Context) error

type Options struct {
	AppName              string
	PostgresCheck        Checker
	PostgresReadCheck    Checker
	RedisCheck           Checker
	ClickHouseCheck      Checker
	ObjectStorageCheck   Checker
	KafkaEnabled         bool
	ObjectStorageEnabled bool
	ClickHouseEnabled    bool
	Build                buildinfo.Info
}

type Service struct {
	opts Options
}

type Liveness struct {
	Status    string    `json:"status"`
	App       string    `json:"app"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
	GitSHA    string    `json:"git_sha,omitempty"`
	BuildTime string    `json:"build_time,omitempty"`
}

type Readiness struct {
	Status       string            `json:"status"`
	App          string            `json:"app"`
	Timestamp    time.Time         `json:"timestamp"`
	Dependencies map[string]string `json:"dependencies"`
	Version      string            `json:"version,omitempty"`
	GitSHA       string            `json:"git_sha,omitempty"`
	BuildTime    string            `json:"build_time,omitempty"`
}

func NewService(opts Options) *Service {
	return &Service{opts: opts}
}

func (s *Service) Live() Liveness {
	return Liveness{
		Status:    "ok",
		App:       s.opts.AppName,
		Timestamp: time.Now().UTC(),
		Version:   s.opts.Build.Version,
		GitSHA:    s.opts.Build.GitSHA,
		BuildTime: s.opts.Build.BuildTime,
	}
}

func (s *Service) Ready(ctx context.Context) Readiness {
	deps := map[string]string{
		"postgres":       "unknown",
		"postgres_read":  "unknown",
		"redis":          "unknown",
		"clickhouse":     "disabled",
		"kafka":          "disabled",
		"object_storage": "disabled",
	}
	status := "ok"

	if s.opts.PostgresCheck != nil {
		if err := s.opts.PostgresCheck(ctx); err != nil {
			status = "degraded"
			deps["postgres"] = "down"
		} else {
			deps["postgres"] = "up"
		}
	}
	if s.opts.PostgresReadCheck != nil {
		if err := s.opts.PostgresReadCheck(ctx); err != nil {
			status = "degraded"
			deps["postgres_read"] = "down"
		} else {
			deps["postgres_read"] = "up"
		}
	}

	if s.opts.RedisCheck != nil {
		if err := s.opts.RedisCheck(ctx); err != nil {
			status = "degraded"
			deps["redis"] = "down"
		} else {
			deps["redis"] = "up"
		}
	}

	if s.opts.ClickHouseEnabled {
		deps["clickhouse"] = "configured"
		if s.opts.ClickHouseCheck != nil {
			if err := s.opts.ClickHouseCheck(ctx); err != nil {
				status = "degraded"
				deps["clickhouse"] = "down"
			} else {
				deps["clickhouse"] = "up"
			}
		}
	}
	if s.opts.KafkaEnabled {
		deps["kafka"] = "configured"
	}
	if s.opts.ObjectStorageEnabled {
		deps["object_storage"] = "configured"
		if s.opts.ObjectStorageCheck != nil {
			if err := s.opts.ObjectStorageCheck(ctx); err != nil {
				status = "degraded"
				deps["object_storage"] = "down"
			} else {
				deps["object_storage"] = "up"
			}
		}
	}

	return Readiness{
		Status:       status,
		App:          s.opts.AppName,
		Timestamp:    time.Now().UTC(),
		Dependencies: deps,
		Version:      s.opts.Build.Version,
		GitSHA:       s.opts.Build.GitSHA,
		BuildTime:    s.opts.Build.BuildTime,
	}
}
