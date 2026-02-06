package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	once sync.Once
	m    *Metrics
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// Request metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.HistogramVec
	ResponseSize    *prometheus.HistogramVec

	// Connection metrics
	ConnectionsTotal   *prometheus.CounterVec
	ConnectionsActive  *prometheus.GaugeVec
	ConnectionDuration *prometheus.HistogramVec

	// Stream metrics
	StreamsTotal   *prometheus.CounterVec
	StreamsActive  *prometheus.GaugeVec
	StreamDuration *prometheus.HistogramVec

	// Frame metrics
	FramesSent     *prometheus.CounterVec
	FramesReceived *prometheus.CounterVec
	FrameSize      *prometheus.HistogramVec

	// Error metrics
	ErrorsTotal *prometheus.CounterVec

	// Bandwidth metrics
	BandwidthBytes *prometheus.CounterVec

	// System metrics
	GoroutinesCount  prometheus.Gauge
	MemoryUsageBytes prometheus.Gauge
}

// Init initializes Prometheus metrics (call once at startup)
func Init() *Metrics {
	once.Do(func() {
		m = &Metrics{
			// Request metrics
			RequestsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_requests_total",
					Help: "Total number of HTTP requests processed",
				},
				[]string{"method", "status", "account"},
			),
			RequestDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tunnel_request_duration_seconds",
					Help:    "HTTP request duration in seconds",
					Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
				},
				[]string{"method", "status"},
			),
			RequestSize: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tunnel_request_size_bytes",
					Help:    "HTTP request size in bytes",
					Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10MB
				},
				[]string{"method"},
			),
			ResponseSize: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tunnel_response_size_bytes",
					Help:    "HTTP response size in bytes",
					Buckets: prometheus.ExponentialBuckets(100, 10, 8),
				},
				[]string{"status"},
			),

			// Connection metrics
			ConnectionsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_connections_total",
					Help: "Total number of connections established",
				},
				[]string{"account"},
			),
			ConnectionsActive: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "tunnel_connections_active",
					Help: "Current number of active connections",
				},
				[]string{"account"},
			),
			ConnectionDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tunnel_connection_duration_seconds",
					Help:    "Connection duration in seconds",
					Buckets: []float64{1, 10, 60, 300, 600, 1800, 3600, 7200}, // 1s to 2h
				},
				[]string{"account"},
			),

			// Stream metrics
			StreamsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_streams_total",
					Help: "Total number of streams created",
				},
				[]string{"account"},
			),
			StreamsActive: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "tunnel_streams_active",
					Help: "Current number of active streams",
				},
				[]string{"account"},
			),
			StreamDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tunnel_stream_duration_seconds",
					Help:    "Stream duration in seconds",
					Buckets: []float64{.01, .05, .1, .5, 1, 5, 10, 30, 60},
				},
				[]string{"account"},
			),

			// Frame metrics
			FramesSent: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_frames_sent_total",
					Help: "Total number of frames sent",
				},
				[]string{"type"},
			),
			FramesReceived: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_frames_received_total",
					Help: "Total number of frames received",
				},
				[]string{"type"},
			),
			FrameSize: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tunnel_frame_size_bytes",
					Help:    "Frame payload size in bytes",
					Buckets: prometheus.ExponentialBuckets(64, 4, 10), // 64B to 16MB
				},
				[]string{"type"},
			),

			// Error metrics
			ErrorsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_errors_total",
					Help: "Total number of errors",
				},
				[]string{"type", "component"},
			),

			// Bandwidth metrics
			BandwidthBytes: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tunnel_bandwidth_bytes_total",
					Help: "Total bandwidth in bytes",
				},
				[]string{"direction", "account"}, // direction: inbound/outbound
			),

			// System metrics
			GoroutinesCount: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "tunnel_goroutines_count",
					Help: "Current number of goroutines",
				},
			),
			MemoryUsageBytes: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "tunnel_memory_usage_bytes",
					Help: "Current memory usage in bytes",
				},
			),
		}
	})
	return m
}

// Get returns the global metrics instance
func Get() *Metrics {
	if m == nil {
		return Init()
	}
	return m
}

// RecordRequest records an HTTP request with duration
func (m *Metrics) RecordRequest(accountID, method, status string, duration time.Duration) {
	m.RequestsTotal.WithLabelValues(method, status, accountID).Inc()
	m.RequestDuration.WithLabelValues(method, status).Observe(duration.Seconds())
}

// RecordRequestSize records request body size
func (m *Metrics) RecordRequestSize(method string, size int64) {
	m.RequestSize.WithLabelValues(method).Observe(float64(size))
}

// RecordResponseSize records response body size
func (m *Metrics) RecordResponseSize(status string, size int64) {
	m.ResponseSize.WithLabelValues(status).Observe(float64(size))
}

// IncrementConnection increments connection counter and gauge
func (m *Metrics) IncrementConnection(accountID string) {
	m.ConnectionsTotal.WithLabelValues(accountID).Inc()
	m.ConnectionsActive.WithLabelValues(accountID).Inc()
}

// DecrementConnection decrements active connections gauge
func (m *Metrics) DecrementConnection(accountID string, duration time.Duration) {
	m.ConnectionsActive.WithLabelValues(accountID).Dec()
	m.ConnectionDuration.WithLabelValues(accountID).Observe(duration.Seconds())
}

// IncrementStream increments stream counter and gauge
func (m *Metrics) IncrementStream(accountID string) {
	m.StreamsTotal.WithLabelValues(accountID).Inc()
	m.StreamsActive.WithLabelValues(accountID).Inc()
}

// DecrementStream decrements active streams gauge
func (m *Metrics) DecrementStream(accountID string, duration time.Duration) {
	m.StreamsActive.WithLabelValues(accountID).Dec()
	m.StreamDuration.WithLabelValues(accountID).Observe(duration.Seconds())
}

// RecordFrameSent records a sent frame
func (m *Metrics) RecordFrameSent(frameType string, size int) {
	m.FramesSent.WithLabelValues(frameType).Inc()
	m.FrameSize.WithLabelValues(frameType).Observe(float64(size))
}

// RecordFrameReceived records a received frame
func (m *Metrics) RecordFrameReceived(frameType string, size int) {
	m.FramesReceived.WithLabelValues(frameType).Inc()
	m.FrameSize.WithLabelValues(frameType).Observe(float64(size))
}

// RecordError records an error
func (m *Metrics) RecordError(errorType, component string) {
	m.ErrorsTotal.WithLabelValues(errorType, component).Inc()
}

// RecordBandwidth records bandwidth usage
func (m *Metrics) RecordBandwidth(direction, accountID string, bytes int64) {
	m.BandwidthBytes.WithLabelValues(direction, accountID).Add(float64(bytes))
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics(goroutines int, memoryBytes uint64) {
	m.GoroutinesCount.Set(float64(goroutines))
	m.MemoryUsageBytes.Set(float64(memoryBytes))
}
