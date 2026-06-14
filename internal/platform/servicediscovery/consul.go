package servicediscovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

type ConsulRegistrar struct {
	cfg    config.ServiceDiscoveryConfig
	client *http.Client
}

func NewConsulRegistrar(cfg config.ServiceDiscoveryConfig) *ConsulRegistrar {
	return &ConsulRegistrar{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (r *ConsulRegistrar) Register(ctx context.Context) error {
	reqBody := consulServiceRegistration{
		ID:      r.cfg.ServiceID,
		Name:    r.cfg.ServiceName,
		Address: r.cfg.ServiceAddress,
		Port:    r.cfg.ServicePort,
		Tags:    consulServiceTags(r.cfg),
		Check: &consulServiceCheck{
			HTTP:                           fmt.Sprintf("http://%s:%d%s", r.cfg.ServiceAddress, r.cfg.ServicePort, r.cfg.HealthCheckPath),
			Interval:                       "10s",
			Timeout:                        "2s",
			DeregisterCriticalServiceAfter: "1m",
		},
	}

	return r.putJSON(ctx, "/v1/agent/service/register", reqBody)
}

func (r *ConsulRegistrar) Deregister(ctx context.Context) error {
	path := fmt.Sprintf("/v1/agent/service/deregister/%s", r.cfg.ServiceID)
	return r.putJSON(ctx, path, nil)
}

func (r *ConsulRegistrar) putJSON(ctx context.Context, path string, body any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(r.cfg.ConsulHTTPAddr, "/")+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("consul request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func consulServiceTags(cfg config.ServiceDiscoveryConfig) []string {
	tags := []string{"sendflow.local"}
	if cfg.ServiceName != "sendflow-api" {
		return tags
	}
	return append(tags,
		"traefik.enable=true",
		"traefik.http.routers.sendflow-api.entrypoints=web",
		"traefik.http.routers.sendflow-api.rule=PathPrefix(`/api/`) || Path(`/openapi.json`) || PathPrefix(`/o/`) || PathPrefix(`/t/`) || PathPrefix(`/u/`)",
		"traefik.http.routers.sendflow-api.middlewares=sendflow-security@file,sendflow-ratelimit@file",
		"traefik.http.routers.sendflow-auth.entrypoints=web",
		"traefik.http.routers.sendflow-auth.rule=Path(`/api/v1/auth/signup`) || Path(`/api/v1/auth/login`) || Path(`/api/v1/auth/login/mfa`) || Path(`/api/v1/auth/password/forgot`) || Path(`/api/v1/auth/password/reset`) || PathPrefix(`/api/v1/auth/oauth/`)",
		"traefik.http.routers.sendflow-auth.priority=100",
		"traefik.http.routers.sendflow-auth.middlewares=sendflow-security@file,sendflow-auth-ratelimit@file",
	)
}

type consulServiceRegistration struct {
	ID      string              `json:"ID"`
	Name    string              `json:"Name"`
	Address string              `json:"Address"`
	Port    int                 `json:"Port"`
	Tags    []string            `json:"Tags"`
	Check   *consulServiceCheck `json:"Check,omitempty"`
}

type consulServiceCheck struct {
	HTTP                           string `json:"HTTP"`
	Interval                       string `json:"Interval"`
	Timeout                        string `json:"Timeout"`
	DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter"`
}
