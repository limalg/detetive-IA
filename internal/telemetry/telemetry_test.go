package telemetry_test

import (
	"testing"
	"time"

	"detetive-ia/internal/telemetry"
)

func TestTelemetryCollectorAndMetrics(t *testing.T) {
	collector := telemetry.GetCollector()

	// 1. Test recording span
	span := telemetry.TraceSpan{
		TraceID:    telemetry.NewTraceID(),
		SpanID:     telemetry.NewSpanID(),
		Name:       "TestSpan",
		Component:  "test_component",
		Status:     "ok",
		StartTime:  time.Now(),
		DurationMs: 42,
	}
	collector.RecordSpan(span)

	spans := collector.GetRecentSpans(10)
	if len(spans) == 0 {
		t.Fatalf("expected at least 1 span recorded, got 0")
	}
	if spans[0].Name != "TestSpan" {
		t.Errorf("expected span name TestSpan, got %s", spans[0].Name)
	}

	// 2. Test logging
	collector.Log("INFO", "test_component", span.TraceID, "Mensagem de log de teste", map[string]interface{}{"foo": "bar"})
	logs := collector.GetRecentLogs(10, "")
	if len(logs) == 0 {
		t.Fatalf("expected at least 1 log entry, got 0")
	}
	if logs[0].Message != "Mensagem de log de teste" {
		t.Errorf("expected log message to match, got %s", logs[0].Message)
	}

	// 3. Test metrics
	metrics := collector.GetMetrics()
	if metrics.TotalRequests < 1 {
		t.Errorf("expected total requests >= 1, got %d", metrics.TotalRequests)
	}
	if metrics.AllocMB <= 0 {
		t.Errorf("expected valid AllocMB metric, got %f", metrics.AllocMB)
	}
}
