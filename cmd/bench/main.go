package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "localhost:6380", "FastKV server address")
	mode := flag.String("mode", "get", "benchmark mode: get, set, or mixed")
	clients := flag.Int("clients", 50, "number of concurrent clients")
	opsPerClient := flag.Int("ops", 1000, "operations per client")
	keyspace := flag.Int("keyspace", 1000, "number of benchmark keys")
	valueSize := flag.Int("value-size", 32, "value size in bytes")
	flag.Parse()

	if *mode != "get" && *mode != "set" && *mode != "mixed" {
		log.Fatalf("invalid mode %q: must be get, set, or mixed", *mode)
	}

	value := strings.Repeat("x", *valueSize)

	if *mode == "get" || *mode == "mixed" {
		log.Printf("preloading %d keys...\n", *keyspace)

		if err := preload(*addr, *keyspace, value); err != nil {
			log.Fatalf("failed to preload keys: %v", err)
		}

		log.Println("preload complete")
	}

	var wg sync.WaitGroup
	var successfulOps int64
	var failedOps int64
	var totalLatencyNanos int64

	start := time.Now()

	for clientID := 0; clientID < *clients; clientID++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			runWorker(
				id,
				*addr,
				*mode,
				*opsPerClient,
				*keyspace,
				value,
				&successfulOps,
				&failedOps,
				&totalLatencyNanos,
			)
		}(clientID)
	}

	wg.Wait()

	elapsed := time.Since(start)

	successes := atomic.LoadInt64(&successfulOps)
	failures := atomic.LoadInt64(&failedOps)
	totalOps := successes + failures

	opsPerSecond := float64(successes) / elapsed.Seconds()

	var avgLatency time.Duration
	if successes > 0 {
		avgLatency = time.Duration(atomic.LoadInt64(&totalLatencyNanos) / successes)
	}

	fmt.Println()
	fmt.Println("FastKV Benchmark Results")
	fmt.Println("------------------------")
	fmt.Printf("Mode:              %s\n", *mode)
	fmt.Printf("Clients:           %d\n", *clients)
	fmt.Printf("Ops/client:        %d\n", *opsPerClient)
	fmt.Printf("Total attempted:   %d\n", totalOps)
	fmt.Printf("Successful ops:    %d\n", successes)
	fmt.Printf("Failed ops:        %d\n", failures)
	fmt.Printf("Duration:          %s\n", elapsed)
	fmt.Printf("Throughput:        %.2f ops/sec\n", opsPerSecond)
	fmt.Printf("Avg latency/op:    %s\n", avgLatency)
}

func preload(addr string, keyspace int, value string) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for i := 0; i < keyspace; i++ {
		key := fmt.Sprintf("bench:%d", i)

		if _, err := fmt.Fprintf(writer, "SET %s %s\n", key, value); err != nil {
			return err
		}

		if err := writer.Flush(); err != nil {
			return err
		}

		response, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		if strings.TrimSpace(response) != "OK" {
			return fmt.Errorf("unexpected preload response: %q", strings.TrimSpace(response))
		}
	}

	return nil
}

func runWorker(
	id int,
	addr string,
	mode string,
	ops int,
	keyspace int,
	value string,
	successfulOps *int64,
	failedOps *int64,
	totalLatencyNanos *int64,
) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		atomic.AddInt64(failedOps, int64(ops))
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

	for i := 0; i < ops; i++ {
		command := buildCommand(id, i, mode, keyspace, value, rng)

		start := time.Now()

		if _, err := fmt.Fprintf(writer, "%s\n", command); err != nil {
			atomic.AddInt64(failedOps, 1)
			continue
		}

		if err := writer.Flush(); err != nil {
			atomic.AddInt64(failedOps, 1)
			continue
		}

		response, err := reader.ReadString('\n')
		if err != nil {
			atomic.AddInt64(failedOps, 1)
			continue
		}

		response = strings.TrimSpace(response)

		if strings.HasPrefix(response, "ERR") {
			atomic.AddInt64(failedOps, 1)
			continue
		}

		latency := time.Since(start)

		atomic.AddInt64(successfulOps, 1)
		atomic.AddInt64(totalLatencyNanos, latency.Nanoseconds())
	}
}

func buildCommand(
	clientID int,
	opID int,
	mode string,
	keyspace int,
	value string,
	rng *rand.Rand,
) string {
	switch mode {
	case "set":
		key := fmt.Sprintf("bench:set:%d:%d", clientID, opID)
		return fmt.Sprintf("SET %s %s", key, value)

	case "get":
		key := fmt.Sprintf("bench:%d", rng.Intn(keyspace))
		return fmt.Sprintf("GET %s", key)

	case "mixed":
		key := fmt.Sprintf("bench:%d", rng.Intn(keyspace))

		if rng.Intn(100) < 80 {
			return fmt.Sprintf("GET %s", key)
		}

		return fmt.Sprintf("SET %s %s", key, value)

	default:
		key := fmt.Sprintf("bench:%d", rng.Intn(keyspace))
		return fmt.Sprintf("GET %s", key)
	}
}
