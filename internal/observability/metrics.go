package observability

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "path", "status"},
	)
	HTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	HTTPInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "http_in_flight_requests", Help: "In-flight HTTP requests"},
		[]string{"method", "path"},
	)
	PaymentsCreated = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "payments_created_total", Help: "Payments created"},
	)
	PaymentsPaid = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "payments_paid_total", Help: "Payments marked paid"},
	)
)

func init() {
	prometheus.MustRegister(HTTPRequests, HTTPDuration, HTTPInFlight, PaymentsCreated, PaymentsPaid)
}

// StartMetricsServer listens on addr in a goroutine (separate from API).
func StartMetricsServer(addr string, log *slog.Logger) {
	if addr == "" {
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Info("metrics listening", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server failed", "err", err)
		}
	}()
}

// NormalizeOTLPEndpoint strips scheme for gRPC dial (host:port).
func NormalizeOTLPEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}
