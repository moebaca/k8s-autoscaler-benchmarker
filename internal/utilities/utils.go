// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package utilities provides helper functions for the k8s-autoscaler-benchmarker application.
package utilities

import (
	"fmt"
	"time"
)

// PrintSummary displays a summary of the benchmark results with colored output for better readability.
// It takes the time duration of various operations and prints them to the standard output.
// The color coding helps in distinguishing between different sections of the summary.
func PrintSummary(provisioningTime, instanceRegistrationTime, podReadinessTime, nodeDeregistrationTime, terminationTime time.Duration) {
	const colorReset = "\033[0m"
	const colorBold = "\033[1m"
	const colorRed = "\033[31m"
	const colorGreen = "\033[32m"
	const colorYellow = "\033[33m"
	const colorCyan = "\033[36m"

	fmt.Printf("\n%s%sBenchmarks Summary%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s--------------------------------------------%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%sInstance Initiation Time:     %s%.2f seconds%s\n", colorBold+colorGreen, colorReset, provisioningTime.Seconds(), colorReset)
	fmt.Printf("%sInstance Registration Time:   %s%.2f seconds%s\n", colorBold+colorGreen, colorReset, instanceRegistrationTime.Seconds(), colorReset)
	fmt.Printf("%sPod Readiness Time:           %s%.2f seconds%s\n", colorBold+colorGreen, colorReset, podReadinessTime.Seconds(), colorReset)
	fmt.Printf("%sInstance Deregistration Time: %s%.2f seconds%s\n", colorBold+colorRed, colorReset, nodeDeregistrationTime.Seconds(), colorReset)
	fmt.Printf("%sInstance Termination Time:    %s%.2f seconds%s\n", colorBold+colorRed, colorReset, terminationTime.Seconds(), colorReset)
	fmt.Printf("%s%s--------------------------------------------%s\n\n", colorBold, colorYellow, colorReset)
}

// Int32Ptr takes an int32 and returns a pointer to it.
// This function is a convenience for situations where a pointer is required.
func Int32Ptr(i int32) *int32 { return &i }
