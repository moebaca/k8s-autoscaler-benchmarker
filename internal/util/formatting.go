// Package util provides utility functions for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package util

import (
	"fmt"
	"time"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/config"
)

// PrintSummary displays benchmark results with colored output.
func PrintSummary(provisioningTime, registrationTime, podReadinessTime, deregistrationTime, terminationTime time.Duration) {
	fmt.Printf("\n%s%sBenchmarks Summary%s\n", config.ColorBold, config.ColorCyan, config.ColorReset)
	fmt.Printf("%s%s--------------------------------------------%s\n", config.ColorBold, config.ColorYellow, config.ColorReset)
	fmt.Printf("%sInstance Initiation Time:     %s%.2f seconds%s\n", config.ColorBold+config.ColorGreen, config.ColorReset, provisioningTime.Seconds(), config.ColorReset)
	fmt.Printf("%sInstance Registration Time:   %s%.2f seconds%s\n", config.ColorBold+config.ColorGreen, config.ColorReset, registrationTime.Seconds(), config.ColorReset)
	fmt.Printf("%sPod Readiness Time:           %s%.2f seconds%s\n", config.ColorBold+config.ColorGreen, config.ColorReset, podReadinessTime.Seconds(), config.ColorReset)
	fmt.Printf("%sInstance Deregistration Time: %s%.2f seconds%s\n", config.ColorBold+config.ColorRed, config.ColorReset, deregistrationTime.Seconds(), config.ColorReset)
	fmt.Printf("%sInstance Termination Time:    %s%.2f seconds%s\n", config.ColorBold+config.ColorRed, config.ColorReset, terminationTime.Seconds(), config.ColorReset)
	fmt.Printf("%s%s--------------------------------------------%s\n\n", config.ColorBold, config.ColorYellow, config.ColorReset)
}

// FormatDuration formats a duration in a human-readable form.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2f seconds", d.Seconds())
	}
	minutes := d / time.Minute
	seconds := (d % time.Minute) / time.Second
	return fmt.Sprintf("%d min %d sec", minutes, seconds)
}
