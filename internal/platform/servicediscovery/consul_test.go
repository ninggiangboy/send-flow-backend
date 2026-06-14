package servicediscovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

func TestConsulRegistrar_Register(t *testing.T) {
	var got consulServiceRegistration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/agent/service/register" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registrar := NewConsulRegistrar(config.ServiceDiscoveryConfig{
		ConsulHTTPAddr:  server.URL,
		ServiceName:     "sendflow-api",
		ServiceID:       "sendflow-api-local",
		ServiceAddress:  "host.docker.internal",
		ServicePort:     8081,
		HealthCheckPath: "/api/readyz",
	})

	if err := registrar.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}

	if got.ID != "sendflow-api-local" {
		t.Fatalf("unexpected service id: %s", got.ID)
	}
	if got.Name != "sendflow-api" {
		t.Fatalf("unexpected service name: %s", got.Name)
	}
	if got.Address != "host.docker.internal" {
		t.Fatalf("unexpected service address: %s", got.Address)
	}
	if got.Port != 8081 {
		t.Fatalf("unexpected service port: %d", got.Port)
	}
	if got.Check == nil || got.Check.HTTP != "http://host.docker.internal:8081/api/readyz" {
		t.Fatalf("unexpected health check: %#v", got.Check)
	}
	if !contains(got.Tags, "traefik.enable=true") {
		t.Fatalf("expected traefik enable tag, got %#v", got.Tags)
	}
}

func TestConsulRegistrar_RegisterWorkerDoesNotUseTraefikTags(t *testing.T) {
	var got consulServiceRegistration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registrar := NewConsulRegistrar(config.ServiceDiscoveryConfig{
		ConsulHTTPAddr:  server.URL,
		ServiceName:     "sendflow-worker",
		ServiceID:       "sendflow-worker-local",
		ServiceAddress:  "host.docker.internal",
		ServicePort:     8082,
		HealthCheckPath: "/api/readyz",
	})

	if err := registrar.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}

	if got.Name != "sendflow-worker" {
		t.Fatalf("unexpected service name: %s", got.Name)
	}
	if contains(got.Tags, "traefik.enable=true") {
		t.Fatalf("worker should not include traefik tags: %#v", got.Tags)
	}
	if !contains(got.Tags, "sendflow.local") {
		t.Fatalf("expected common sendflow tag, got %#v", got.Tags)
	}
}

func TestConsulRegistrar_Deregister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/agent/service/deregister/sendflow-api-local" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registrar := NewConsulRegistrar(config.ServiceDiscoveryConfig{
		ConsulHTTPAddr: server.URL,
		ServiceID:      "sendflow-api-local",
	})

	if err := registrar.Deregister(context.Background()); err != nil {
		t.Fatalf("deregister: %v", err)
	}
}

func TestConsulRegistrar_ReturnsStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	registrar := NewConsulRegistrar(config.ServiceDiscoveryConfig{
		ConsulHTTPAddr: server.URL,
		ServiceID:      "sendflow-api-local",
	})

	if err := registrar.Deregister(context.Background()); err == nil {
		t.Fatal("expected status error")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
