package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// TraceSpan represents an OpenTelemetry-compatible span.
type TraceSpan struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Name       string                 `json:"name"`
	Component  string                 `json:"component"`
	Status     string                 `json:"status"` // "ok", "error"
	StartTime  time.Time              `json:"start_time"`
	DurationMs int64                  `json:"duration_ms"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// LogEntry represents a structured log entry.
type LogEntry struct {
	ID         int64                  `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"` // "INFO", "WARN", "ERROR", "DEBUG"
	Component  string                 `json:"component"`
	TraceID    string                 `json:"trace_id,omitempty"`
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// SystemMetrics represents live performance and application metrics.
type SystemMetrics struct {
	TotalRequests   int64   `json:"total_requests"`
	TotalErrors     int64   `json:"total_errors"`
	TotalAICalls    int64   `json:"total_ai_calls"`
	TotalAIErrors   int64   `json:"total_ai_errors"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	AllocMB         float64 `json:"alloc_mb"`
	SysMB           float64 `json:"sys_mb"`
	NumGoroutines   int     `json:"num_goroutines"`
	MemoryThreshold string  `json:"memory_threshold"`
}

// Collector holds in-memory traces and logs using circular buffers.
type Collector struct {
	mu           sync.RWMutex
	logs         []LogEntry
	spans        []TraceSpan
	maxLogs      int
	maxSpans     int
	logIDCounter int64

	totalRequests int64
	totalErrors   int64
	totalAICalls  int64
	totalAIErrors int64
	totalLatency  int64
}

var globalCollector *Collector
var once sync.Once

// GetCollector returns the singleton telemetry collector.
func GetCollector() *Collector {
	once.Do(func() {
		globalCollector = &Collector{
			logs:     make([]LogEntry, 0, 1000),
			spans:    make([]TraceSpan, 0, 500),
			maxLogs:  1000,
			maxSpans: 500,
		}
	})
	return globalCollector
}

func generateID(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewTraceID generates a 16-byte hex trace ID.
func NewTraceID() string {
	return generateID(16)
}

// NewSpanID generates an 8-byte hex span ID.
func NewSpanID() string {
	return generateID(8)
}

// RecordSpan saves a span to the circular buffer.
func (c *Collector) RecordSpan(span TraceSpan) {
	c.mu.Lock()
	defer c.mu.Unlock()

	atomic.AddInt64(&c.totalRequests, 1)
	atomic.AddInt64(&c.totalLatency, span.DurationMs)
	if span.Status == "error" {
		atomic.AddInt64(&c.totalErrors, 1)
	}
	if span.Component == "ai_client" {
		atomic.AddInt64(&c.totalAICalls, 1)
		if span.Status == "error" {
			atomic.AddInt64(&c.totalAIErrors, 1)
		}
	}

	if len(c.spans) >= c.maxSpans {
		c.spans = c.spans[1:]
	}
	c.spans = append(c.spans, span)
}

// Log records a structured log entry.
func (c *Collector) Log(level, component, traceID, msg string, attrs map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logIDCounter++
	entry := LogEntry{
		ID:         c.logIDCounter,
		Timestamp:  time.Now(),
		Level:      level,
		Component:  component,
		TraceID:    traceID,
		Message:    msg,
		Attributes: attrs,
	}

	if len(c.logs) >= c.maxLogs {
		c.logs = c.logs[1:]
	}
	c.logs = append(c.logs, entry)
}

// GetRecentLogs returns up to limit logs in descending order.
func (c *Collector) GetRecentLogs(limit int, levelFilter string) []LogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.logs) {
		limit = len(c.logs)
	}

	result := make([]LogEntry, 0, limit)
	for i := len(c.logs) - 1; i >= 0 && len(result) < limit; i-- {
		log := c.logs[i]
		if levelFilter == "" || log.Level == levelFilter {
			result = append(result, log)
		}
	}
	return result
}

// GetRecentSpans returns recent OpenTelemetry spans.
func (c *Collector) GetRecentSpans(limit int) []TraceSpan {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.spans) {
		limit = len(c.spans)
	}

	result := make([]TraceSpan, 0, limit)
	for i := len(c.spans) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, c.spans[i])
	}
	return result
}

// GetMetrics returns combined system and OpenTelemetry performance statistics.
func (c *Collector) GetMetrics() SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	reqs := atomic.LoadInt64(&c.totalRequests)
	latency := atomic.LoadInt64(&c.totalLatency)
	avgLatency := 0.0
	if reqs > 0 {
		avgLatency = float64(latency) / float64(reqs)
	}

	return SystemMetrics{
		TotalRequests:   reqs,
		TotalErrors:     atomic.LoadInt64(&c.totalErrors),
		TotalAICalls:    atomic.LoadInt64(&c.totalAICalls),
		TotalAIErrors:   atomic.LoadInt64(&c.totalAIErrors),
		AvgLatencyMs:    avgLatency,
		AllocMB:         float64(m.Alloc) / (1024 * 1024),
		SysMB:           float64(m.Sys) / (1024 * 1024),
		NumGoroutines:   runtime.NumGoroutine(),
		MemoryThreshold: "< 60MB RAM Target (<1GB host footprint)",
	}
}

// Global convenience functions:
func Info(component, traceID, msg string, attrs map[string]interface{}) {
	GetCollector().Log("INFO", component, traceID, msg, attrs)
}

func Warn(component, traceID, msg string, attrs map[string]interface{}) {
	GetCollector().Log("WARN", component, traceID, msg, attrs)
}

func Error(component, traceID, msg string, attrs map[string]interface{}) {
	GetCollector().Log("ERROR", component, traceID, msg, attrs)
}

func Record(span TraceSpan) {
	GetCollector().RecordSpan(span)
}

func Metrics() SystemMetrics {
	return GetCollector().GetMetrics()
}
