package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

var (
	endpoint      = flag.String("endpoint", "localhost:4317", "OTLP gRPC endpoint")
	duration      = flag.Duration("duration", 60*time.Second, "Test duration")
	ratePerSecond = flag.Int("rate", 10000, "Target spans per second")
	numWorkers    = flag.Int("workers", 10, "Number of concurrent workers")
	batchSize     = flag.Int("batch", 100, "Spans per batch")
)

type Stats struct {
	spansSent     atomic.Uint64
	spansSucceeded atomic.Uint64
	spansFailed   atomic.Uint64
	totalLatency  atomic.Uint64
	requests      atomic.Uint64
}

func main() {
	flag.Parse()

	log.Printf("Starting load test:")
	log.Printf("  Endpoint: %s", *endpoint)
	log.Printf("  Duration: %s", *duration)
	log.Printf("  Target rate: %d spans/sec", *ratePerSecond)
	log.Printf("  Workers: %d", *numWorkers)
	log.Printf("  Batch size: %d", *batchSize)

	// Connect to collector
	conn, err := grpc.Dial(*endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := coltracepb.NewTraceServiceClient(conn)

	stats := &Stats{}
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// Start stats reporter
	go reportStats(ctx, stats)

	// Start workers
	var wg sync.WaitGroup
	spansPerWorker := *ratePerSecond / *numWorkers
	tickInterval := time.Second / time.Duration(spansPerWorker / *batchSize)

	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorker(ctx, client, workerID, tickInterval, stats)
		}(i)
	}

	wg.Wait()

	// Print final stats
	printFinalStats(stats)
}

func runWorker(ctx context.Context, client coltracepb.TraceServiceClient, workerID int, tickInterval time.Duration, stats *Stats) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendBatch(ctx, client, workerID, stats)
		}
	}
}

func sendBatch(ctx context.Context, client coltracepb.TraceServiceClient, workerID int, stats *Stats) {
	spans := generateSpans(*batchSize, workerID)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: fmt.Sprintf("benchmark-service-%d", workerID),
								},
							},
						},
						{
							Key: "service.namespace",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "benchmark",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "benchmark",
							Version: "1.0.0",
						},
						Spans: spans,
					},
				},
			},
		},
	}

	start := time.Now()
	_, err := client.Export(ctx, req)
	latency := time.Since(start)

	stats.requests.Add(1)
	stats.totalLatency.Add(uint64(latency.Milliseconds()))
	stats.spansSent.Add(uint64(len(spans)))

	if err != nil {
		stats.spansFailed.Add(uint64(len(spans)))
	} else {
		stats.spansSucceeded.Add(uint64(len(spans)))
	}
}

func generateSpans(count int, workerID int) []*tracepb.Span {
	spans := make([]*tracepb.Span, count)
	now := time.Now()

	for i := 0; i < count; i++ {
		traceID := randomBytes(16)
		spanID := randomBytes(8)
		parentSpanID := randomBytes(8)

		duration := time.Duration(rand.Int63n(1000)) * time.Millisecond
		startTime := now.Add(-duration)
		endTime := now

		spans[i] = &tracepb.Span{
			TraceId:           traceID,
			SpanId:            spanID,
			ParentSpanId:      parentSpanID,
			Name:              fmt.Sprintf("operation-%d", rand.Intn(100)),
			Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
			StartTimeUnixNano: uint64(startTime.UnixNano()),
			EndTimeUnixNano:   uint64(endTime.UnixNano()),
			Status: &tracepb.Status{
				Code: tracepb.Status_STATUS_CODE_OK,
			},
			Attributes: []*commonpb.KeyValue{
				{
					Key: "http.method",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{
							StringValue: "GET",
						},
					},
				},
				{
					Key: "http.status_code",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_IntValue{
							IntValue: 200,
						},
					},
				},
			},
		}
	}

	return spans
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func reportStats(ctx context.Context, stats *Stats) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastSpans := uint64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentSpans := stats.spansSucceeded.Load()
			currentTime := time.Now()
			elapsed := currentTime.Sub(lastTime).Seconds()

			rate := float64(currentSpans-lastSpans) / elapsed
			avgLatency := float64(0)
			if req := stats.requests.Load(); req > 0 {
				avgLatency = float64(stats.totalLatency.Load()) / float64(req)
			}

			log.Printf("Rate: %.0f spans/sec | Succeeded: %d | Failed: %d | Avg Latency: %.2f ms",
				rate,
				stats.spansSucceeded.Load(),
				stats.spansFailed.Load(),
				avgLatency,
			)

			lastSpans = currentSpans
			lastTime = currentTime
		}
	}
}

func printFinalStats(stats *Stats) {
	fmt.Println("\n=== Final Statistics ===")
	fmt.Printf("Total spans sent: %d\n", stats.spansSent.Load())
	fmt.Printf("Spans succeeded: %d\n", stats.spansSucceeded.Load())
	fmt.Printf("Spans failed: %d\n", stats.spansFailed.Load())
	fmt.Printf("Success rate: %.2f%%\n", float64(stats.spansSucceeded.Load())/float64(stats.spansSent.Load())*100)

	if req := stats.requests.Load(); req > 0 {
		avgLatency := float64(stats.totalLatency.Load()) / float64(req)
		fmt.Printf("Average request latency: %.2f ms\n", avgLatency)
	}
}
