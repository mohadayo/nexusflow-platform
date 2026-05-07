package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type HealthResponse struct {
	Status    string  `json:"status"`
	Service   string  `json:"service"`
	Timestamp float64 `json:"timestamp"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type RouteConfig struct {
	Path    string `json:"path"`
	Target  string `json:"target"`
	Methods string `json:"methods"`
}

type Gateway struct {
	eventBusURL  string
	dashboardURL string
	logger       *log.Logger
}

func NewGateway() *Gateway {
	eventBusURL := os.Getenv("EVENT_BUS_URL")
	if eventBusURL == "" {
		eventBusURL = "http://localhost:5001"
	}
	dashboardURL := os.Getenv("DASHBOARD_URL")
	if dashboardURL == "" {
		dashboardURL = "http://localhost:3000"
	}

	return &Gateway{
		eventBusURL:  eventBusURL,
		dashboardURL: dashboardURL,
		logger:       log.New(os.Stdout, "[api-gateway] ", log.LstdFlags),
	}
}

func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "healthy",
		Service:   "api-gateway",
		Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (g *Gateway) routesHandler(w http.ResponseWriter, r *http.Request) {
	routes := []RouteConfig{
		{Path: "/api/events/*", Target: g.eventBusURL, Methods: "GET,POST"},
		{Path: "/api/channels/*", Target: g.eventBusURL, Methods: "GET,POST"},
		{Path: "/api/subscribe", Target: g.eventBusURL, Methods: "POST"},
		{Path: "/api/subscribers", Target: g.eventBusURL, Methods: "GET"},
		{Path: "/api/stats", Target: g.eventBusURL, Methods: "GET"},
		{Path: "/api/dashboard/*", Target: g.dashboardURL, Methods: "GET"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"routes": routes})
}

func (g *Gateway) proxyHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var targetURL string

	switch {
	case matchPrefix(path, "/api/channels"):
		targetURL = g.eventBusURL + path[4:]
	case matchPrefix(path, "/api/publish"):
		targetURL = g.eventBusURL + path[4:]
	case matchPrefix(path, "/api/subscribe"):
		targetURL = g.eventBusURL + path[4:]
	case matchPrefix(path, "/api/subscribers"):
		targetURL = g.eventBusURL + path[4:]
	case matchPrefix(path, "/api/events"):
		targetURL = g.eventBusURL + path[4:]
	case matchPrefix(path, "/api/stats"):
		targetURL = g.eventBusURL + path[4:]
	default:
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Route not found"})
		return
	}

	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	g.logger.Printf("Proxying %s %s -> %s", r.Method, path, targetURL)

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		g.logger.Printf("Error creating proxy request: %v", err)
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to create proxy request"})
		return
	}

	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	proxyReq.Header.Set("X-Gateway-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		g.logger.Printf("Error proxying request: %v", err)
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Upstream service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.Printf("Error reading response: %v", err)
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read upstream response"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Gateway", "nexusflow")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (g *Gateway) upstreamHealthHandler(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{
		"event-bus": g.eventBusURL + "/health",
		"dashboard": g.dashboardURL + "/health",
	}

	results := make(map[string]interface{})
	client := &http.Client{Timeout: 5 * time.Second}

	for name, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			results[name] = map[string]string{"status": "unreachable", "error": err.Error()}
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			results[name] = map[string]string{"status": "healthy"}
		} else {
			results[name] = map[string]string{"status": "unhealthy", "code": fmt.Sprintf("%d", resp.StatusCode)}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"upstream": results})
}

func matchPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	gw := NewGateway()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", gw.healthHandler)
	mux.HandleFunc("/routes", gw.routesHandler)
	mux.HandleFunc("/upstream/health", gw.upstreamHealthHandler)
	mux.HandleFunc("/api/", gw.proxyHandler)

	handler := corsMiddleware(mux)
	handler = loggingMiddleware(gw.logger, handler)

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}

	gw.logger.Printf("Starting API Gateway on port %s", port)
	gw.logger.Printf("Event Bus URL: %s", gw.eventBusURL)
	gw.logger.Printf("Dashboard URL: %s", gw.dashboardURL)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		gw.logger.Fatalf("Failed to start server: %v", err)
	}
}
