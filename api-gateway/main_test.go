package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	gw := NewGateway()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	gw.healthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body HealthResponse
	json.NewDecoder(resp.Body).Decode(&body)

	if body.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", body.Status)
	}
	if body.Service != "api-gateway" {
		t.Errorf("expected service 'api-gateway', got '%s'", body.Service)
	}
	if body.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestRoutesHandler(t *testing.T) {
	gw := NewGateway()
	req := httptest.NewRequest("GET", "/routes", nil)
	w := httptest.NewRecorder()

	gw.routesHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string][]RouteConfig
	json.NewDecoder(resp.Body).Decode(&body)

	routes := body["routes"]
	if len(routes) == 0 {
		t.Error("expected non-empty routes list")
	}
}

func TestProxyHandlerNotFound(t *testing.T) {
	gw := NewGateway()
	req := httptest.NewRequest("GET", "/api/unknown/path", nil)
	w := httptest.NewRecorder()

	gw.proxyHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestProxyHandlerUpstreamUnavailable(t *testing.T) {
	gw := &Gateway{
		eventBusURL:  "http://localhost:59999",
		dashboardURL: "http://localhost:59998",
		logger:       NewGateway().logger,
	}

	req := httptest.NewRequest("GET", "/api/channels", nil)
	w := httptest.NewRecorder()

	gw.proxyHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", resp.StatusCode)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204 for OPTIONS, got %d", resp.StatusCode)
	}

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected CORS origin '*', got '%s'", origin)
	}
}

func TestCORSMiddlewareNonOptions(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestMatchPrefix(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   bool
	}{
		{"/api/channels", "/api/channels", true},
		{"/api/channels/test", "/api/channels", true},
		{"/api/events", "/api/channels", false},
		{"/short", "/longprefix", false},
	}

	for _, tt := range tests {
		got := matchPrefix(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("matchPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["key"] != "value" {
		t.Errorf("expected 'value', got '%s'", body["key"])
	}
}

func TestUpstreamHealthHandler(t *testing.T) {
	gw := &Gateway{
		eventBusURL:  "http://localhost:59999",
		dashboardURL: "http://localhost:59998",
		logger:       NewGateway().logger,
	}

	req := httptest.NewRequest("GET", "/upstream/health", nil)
	w := httptest.NewRecorder()

	gw.upstreamHealthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	upstream := body["upstream"].(map[string]interface{})
	for _, v := range upstream {
		svc := v.(map[string]interface{})
		if svc["status"] != "unreachable" {
			t.Errorf("expected 'unreachable' for unavailable service, got '%s'", svc["status"])
		}
	}
}
