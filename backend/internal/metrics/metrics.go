package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "booking_http_requests_total",
			Help: "Total number of completed HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "booking_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	BusinessEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "booking_business_events_total",
			Help: "Total number of important business events.",
		},
		[]string{"event"},
	)
)

func init() {
	prometheus.MustRegister(HTTPRequestTotal)
	prometheus.MustRegister(HTTPRequestDuration)
	prometheus.MustRegister(BusinessEventsTotal)
}

func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	statusLabel := strconv.Itoa(status)
	HTTPRequestTotal.WithLabelValues(method, path, statusLabel).Inc()
	HTTPRequestDuration.WithLabelValues(method, path, statusLabel).Observe(duration.Seconds())
}

func RecordBusinessEvent(event string) {
	BusinessEventsTotal.WithLabelValues(event).Inc()
}
