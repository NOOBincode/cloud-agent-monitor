package main

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "checkout_requests_total",
			Help: "Checkout API requests",
		},
		[]string{"path", "status"},
	)
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "checkout_request_duration_seconds",
			Help:    "Checkout request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
)

type chaosConfig struct {
	mu        sync.Mutex
	LatencyMS int `json:"latency_ms"`
	ErrorPct  int `json:"error_pct"` // 0-100, probability of 500 on /api/checkout
}

func (c *chaosConfig) snapshot() (latency int, errPct int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LatencyMS, c.ErrorPct
}

func (c *chaosConfig) set(latency, errPct int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if latency < 0 {
		latency = 0
	}
	if errPct < 0 {
		errPct = 0
	}
	if errPct > 100 {
		errPct = 100
	}
	c.LatencyMS = latency
	c.ErrorPct = errPct
}

func (c *chaosConfig) reset() {
	c.set(0, 0)
}

var chaos chaosConfig

func chaosToken() string {
	t := os.Getenv("CHAOS_TOKEN")
	if t == "" {
		t = "dev-chaos-token"
	}
	return t
}

func requireChaosAuth(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-Chaos-Token") != chaosToken() {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
	}))
	slog.SetDefault(log)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/checkout", checkoutHandler)
	mux.HandleFunc("/internal/chaos", chaosSetHandler)
	mux.HandleFunc("/internal/chaos/reset", chaosResetHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("checkout-sim: use /api/checkout, metrics on /metrics"))
	})

	addr := ":18080"
	if v := os.Getenv("CHECKOUT_SIM_ADDR"); v != "" {
		addr = v
	}
	slog.Info("checkout-sim listening", "addr", addr, "chaos_token_set", os.Getenv("CHAOS_TOKEN") != "")
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server exit", "err", err)
		os.Exit(1)
	}
}

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := "/api/checkout"
	start := time.Now()
	latencyMS, errPct := chaos.snapshot()
	if latencyMS > 0 {
		time.Sleep(time.Duration(latencyMS) * time.Millisecond)
	}
	status := "200"
	if errPct > 0 && rand.Intn(100) < errPct {
		status = "500"
		requestsTotal.WithLabelValues(path, status).Inc()
		requestDuration.WithLabelValues(path).Observe(time.Since(start).Seconds())
		http.Error(w, "checkout failed (injected)", http.StatusInternalServerError)
		return
	}
	requestsTotal.WithLabelValues(path, status).Inc()
	requestDuration.WithLabelValues(path).Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"order_id":"sim-` + strconv.FormatInt(time.Now().UnixNano(), 10) + `"}`))
}

func chaosSetHandler(w http.ResponseWriter, r *http.Request) {
	if !requireChaosAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		LatencyMS int `json:"latency_ms"`
		ErrorPct  int `json:"error_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// allow query string fallback for curl simplicity
		if q := r.URL.Query(); q.Get("latency_ms") != "" || q.Get("error_pct") != "" {
			body.LatencyMS, _ = strconv.Atoi(q.Get("latency_ms"))
			body.ErrorPct, _ = strconv.Atoi(q.Get("error_pct"))
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	chaos.set(body.LatencyMS, body.ErrorPct)
	slog.Warn("chaos updated", "latency_ms", body.LatencyMS, "error_pct", body.ErrorPct)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"latency_ms": body.LatencyMS,
		"error_pct":  body.ErrorPct,
	})
}

func chaosResetHandler(w http.ResponseWriter, r *http.Request) {
	if !requireChaosAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	chaos.reset()
	slog.Info("chaos reset")
	w.WriteHeader(http.StatusNoContent)
}
