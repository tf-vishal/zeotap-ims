package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var components = []string{
	"svc-auth", "svc-payments", "svc-gateway", "svc-users", "svc-notifications",
	"svc-inventory", "svc-search", "svc-analytics", "svc-billing", "svc-cache",
}

var severities = []string{"critical", "warning", "info"}
var signalTypes = []string{"error", "timeout", "panic", "latency_spike", "connection_refused"}

func main() {
	url := flag.String("url", "http://localhost:8080/api/v1/signals", "Target API URL")
	concurrency := flag.Int("c", 100, "Number of concurrent workers")
	total := flag.Int("n", 10000, "Total number of requests to send")
	flag.Parse()

	fmt.Printf("🚀 Starting High-Throughput Load Test\n")
	fmt.Printf("═══════════════════════════════════════════════════\n")
	fmt.Printf("Target URL:  %s\n", *url)
	fmt.Printf("Concurrency: %d workers\n", *concurrency)
	fmt.Printf("Total Req:   %d signals\n", *total)
	fmt.Printf("Components:  %d distinct services\n\n", len(components))

	var success int64
	var failed int64
	var rateLimited int64

	// Optimized HTTP Client for High Throughput
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        2000,
			MaxIdleConnsPerHost: 2000,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
		},
		Timeout: 15 * time.Second,
	}

	batchSize := 500
	numBatches := *total / batchSize
	if *total%batchSize != 0 {
		numBatches++
	}

	reqChan := make(chan struct{}, numBatches)
	for i := 0; i < numBatches; i++ {
		reqChan <- struct{}{}
	}
	close(reqChan)

	var wg sync.WaitGroup
	start := time.Now()

	// Spawn workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range reqChan {
				var payloads []string
				for b := 0; b < batchSize; b++ {
					comp := components[rand.Intn(len(components))]
					sev := severities[rand.Intn(len(severities))]
					stype := signalTypes[rand.Intn(len(signalTypes))]
					ts := time.Now().UTC().Format(time.RFC3339Nano)

					payloads = append(payloads, fmt.Sprintf(
						`{"component_id":"%s","signal_type":"%s","severity":"%s","timestamp":"%s","metadata":{"source":"load_test","batch":true}}`,
						comp, stype, sev, ts,
					))
				}
				
				batchPayload := "[" + strings.Join(payloads, ",") + "]"

				var resp *http.Response
				var err error
				for attempt := 0; attempt < 2; attempt++ {
					req, _ := http.NewRequest("POST", *url, bytes.NewReader([]byte(batchPayload)))
					req.Header.Set("Content-Type", "application/json")
					resp, err = client.Do(req)
					if err == nil {
						break
					}
					if attempt == 0 {
						time.Sleep(50 * time.Millisecond)
					}
				}

				if err != nil {
					atomic.AddInt64(&failed, int64(batchSize))
					if atomic.LoadInt64(&failed) <= int64(batchSize*2) {
						fmt.Printf("Error: %v\n", err)
					}
					continue
				}
				
				if resp.StatusCode == 200 || resp.StatusCode == 202 {
					atomic.AddInt64(&success, int64(batchSize))
				} else if resp.StatusCode == 429 {
					atomic.AddInt64(&rateLimited, int64(batchSize))
				} else {
					atomic.AddInt64(&failed, int64(batchSize))
					if atomic.LoadInt64(&failed) <= int64(batchSize*2) {
						fmt.Printf("HTTP Error: %d\n", resp.StatusCode)
					}
				}
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	rps := float64(*total) / duration.Seconds()

	fmt.Printf("═══════════════════════════════════════════════════\n")
	fmt.Printf("✅ Test Completed in %v\n", duration)
	fmt.Printf("📊 Throughput:    %.2f sig/sec\n", rps)
	fmt.Printf("🟢 Success:       %d\n", success)
	if rateLimited > 0 {
		fmt.Printf("🟡 Rate Limited:  %d (HTTP 429)\n", rateLimited)
	} else {
		fmt.Printf("🟡 Rate Limited:  0\n")
	}
	fmt.Printf("🔴 Failed/Errors: %d\n", failed)
	fmt.Printf("═══════════════════════════════════════════════════\n")
}
