package metrics

import (
	"runtime"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsInit(t *testing.T) {
	m := Init()
	if m == nil {
		t.Fatal("Metrics should not be nil")
	}

	// Verify all metrics are initialized
	if m.RequestsTotal == nil {
		t.Error("RequestsTotal should be initialized")
	}
	if m.ConnectionsActive == nil {
		t.Error("ConnectionsActive should be initialized")
	}
	if m.ErrorsTotal == nil {
		t.Error("ErrorsTotal should be initialized")
	}
}

func TestMetricsGet(t *testing.T) {
	m1 := Get()
	m2 := Get()

	// Should return same instance (singleton)
	if m1 != m2 {
		t.Error("Get() should return same instance")
	}
}

func TestRecordRequest(t *testing.T) {
	m := Init()

	// Record a request
	m.RecordRequest("acc-1", "GET", "200", 100*time.Millisecond)

	// Verify counter incremented
	count := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "200", "acc-1"))
	if count != 1 {
		t.Errorf("Expected count 1, got %f", count)
	}
}

func TestIncrementDecrementConnection(t *testing.T) {
	m := Init()

	// Increment
	m.IncrementConnection("acc-1")

	active := testutil.ToFloat64(m.ConnectionsActive.WithLabelValues("acc-1"))
	if active != 1 {
		t.Errorf("Expected active connections 1, got %f", active)
	}

	// Decrement
	m.DecrementConnection("acc-1", 5*time.Second)

	active = testutil.ToFloat64(m.ConnectionsActive.WithLabelValues("acc-1"))
	if active != 0 {
		t.Errorf("Expected active connections 0, got %f", active)
	}
}

func TestIncrementDecrementStream(t *testing.T) {
	m := Init()

	// Increment
	m.IncrementStream("acc-1")

	active := testutil.ToFloat64(m.StreamsActive.WithLabelValues("acc-1"))
	if active != 1 {
		t.Errorf("Expected active streams 1, got %f", active)
	}

	// Decrement
	m.DecrementStream("acc-1", 100*time.Millisecond)

	active = testutil.ToFloat64(m.StreamsActive.WithLabelValues("acc-1"))
	if active != 0 {
		t.Errorf("Expected active streams 0, got %f", active)
	}
}

func TestRecordFrame(t *testing.T) {
	m := Init()

	// Record sent frame
	m.RecordFrameSent("data", 1024)

	sent := testutil.ToFloat64(m.FramesSent.WithLabelValues("data"))
	if sent != 1 {
		t.Errorf("Expected frames sent 1, got %f", sent)
	}

	// Record received frame
	m.RecordFrameReceived("data", 2048)

	received := testutil.ToFloat64(m.FramesReceived.WithLabelValues("data"))
	if received != 1 {
		t.Errorf("Expected frames received 1, got %f", received)
	}
}

func TestRecordError(t *testing.T) {
	m := Init()

	m.RecordError("connection", "manager")

	errors := testutil.ToFloat64(m.ErrorsTotal.WithLabelValues("connection", "manager"))
	if errors != 1 {
		t.Errorf("Expected errors 1, got %f", errors)
	}
}

func TestRecordBandwidth(t *testing.T) {
	m := Init()

	m.RecordBandwidth("inbound", "acc-1", 1024)
	m.RecordBandwidth("inbound", "acc-1", 2048)

	bandwidth := testutil.ToFloat64(m.BandwidthBytes.WithLabelValues("inbound", "acc-1"))
	if bandwidth != 3072 {
		t.Errorf("Expected bandwidth 3072, got %f", bandwidth)
	}
}

func TestUpdateSystemMetrics(t *testing.T) {
	m := Init()

	goroutines := runtime.NumGoroutine()
	m.UpdateSystemMetrics(goroutines, 1024*1024)

	count := testutil.ToFloat64(m.GoroutinesCount)
	if count != float64(goroutines) {
		t.Errorf("Expected goroutines %d, got %f", goroutines, count)
	}

	memory := testutil.ToFloat64(m.MemoryUsageBytes)
	if memory != 1024*1024 {
		t.Errorf("Expected memory 1048576, got %f", memory)
	}
}

func TestRecordRequestSize(t *testing.T) {
	m := Init()

	m.RecordRequestSize("POST", 5000)

	// For histograms, we can't easily test with testutil.ToFloat64
	// Just verify no panic occurred
	if m.RequestSize == nil {
		t.Error("RequestSize histogram should not be nil")
	}
}

func TestRecordResponseSize(t *testing.T) {
	m := Init()

	m.RecordResponseSize("200", 10000)

	// For histograms, we can't easily test with testutil.ToFloat64
	// Just verify no panic occurred
	if m.ResponseSize == nil {
		t.Error("ResponseSize histogram should not be nil")
	}
}

func TestConcurrentMetrics(t *testing.T) {
	m := Init()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			m.RecordRequest("acc-1", "GET", "200", time.Millisecond)
			m.IncrementConnection("acc-1")
			m.RecordError("test", "component")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	requests := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "200", "acc-1"))
	if requests != 10 {
		t.Errorf("Expected 10 requests, got %f", requests)
	}
}

func BenchmarkRecordRequest(b *testing.B) {
	m := Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordRequest("acc-1", "GET", "200", time.Millisecond)
	}
}

func BenchmarkIncrementConnection(b *testing.B) {
	m := Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.IncrementConnection("acc-1")
	}
}

func BenchmarkRecordFrame(b *testing.B) {
	m := Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordFrameSent("data", 1024)
	}
}
