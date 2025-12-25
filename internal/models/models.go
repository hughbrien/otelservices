package models

import (
	"time"
)

// Metric represents an OpenTelemetry metric
type Metric struct {
	Timestamp                   time.Time
	MetricName                  string
	MetricType                  string
	Value                       float64
	ServiceName                 string
	ServiceNamespace            string
	ServiceInstanceID           string
	DeploymentEnvironment       string
	Attributes                  map[string]string
	ResourceAttributes          map[string]string
	BucketCounts                []uint64
	ExplicitBounds              []float64
	InstrumentationScopeName    string
	InstrumentationScopeVersion string
}

// LogRecord represents an OpenTelemetry log record
type LogRecord struct {
	Timestamp                   time.Time
	ObservedTimestamp           time.Time
	SeverityNumber              uint8
	SeverityText                string
	Body                        string
	BodyType                    string
	ServiceName                 string
	ServiceNamespace            string
	ServiceInstanceID           string
	DeploymentEnvironment       string
	HostName                    string
	TraceID                     string
	SpanID                      string
	TraceFlags                  uint8
	Attributes                  map[string]string
	ResourceAttributes          map[string]string
	InstrumentationScopeName    string
	InstrumentationScopeVersion string
}

// Span represents an OpenTelemetry trace span
type Span struct {
	Timestamp                   time.Time
	TraceID                     string
	SpanID                      string
	ParentSpanID                string
	SpanName                    string
	SpanKind                    string
	StartTime                   time.Time
	EndTime                     time.Time
	DurationNs                  uint64
	StatusCode                  string
	StatusMessage               string
	ServiceName                 string
	ServiceNamespace            string
	ServiceInstanceID           string
	DeploymentEnvironment       string
	Attributes                  map[string]string
	ResourceAttributes          map[string]string
	Events                      []SpanEvent
	Links                       []SpanLink
	InstrumentationScopeName    string
	InstrumentationScopeVersion string
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Timestamp  time.Time
	Name       string
	Attributes map[string]string
}

// SpanLink represents a link to another span
type SpanLink struct {
	TraceID    string
	SpanID     string
	TraceState string
	Attributes map[string]string
}

// TraceIndex represents metadata about a complete trace
type TraceIndex struct {
	TraceID          string
	MinTimestamp     time.Time
	MaxTimestamp     time.Time
	ServiceNames     []string
	RootServiceName  string
	RootSpanName     string
	DurationNs       uint64
	SpanCount        uint32
	HasErrors        bool
}