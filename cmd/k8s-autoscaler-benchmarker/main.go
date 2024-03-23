// Package main is the entry point for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/bench"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/config"
)

func main() {
	// Parse configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context with cancellation for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	setupSignalHandler(cancel)

	// Run the benchmark
	if err := bench.RunBenchmark(ctx, cfg); err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}
}

// setupSignalHandler captures interrupt signals for clean shutdown
func setupSignalHandler(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()
}
