// Package bench provides benchmark orchestration for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/config"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/util"
)

// BenchmarkReport contains full details about a benchmark run.
type BenchmarkReport struct {
	// Benchmark metadata
	Timestamp  time.Time `json:"timestamp"`
	AutoScaler string    `json:"autoscaler"`
	Config     struct {
		Namespace      string `json:"namespace"`
		Replicas       int    `json:"replicas"`
		CPURequest     string `json:"cpu_request"`
		ContainerImage string `json:"container_image"`
	} `json:"config"`

	// Timing results
	Results struct {
		ProvisioningTimeSeconds   float64 `json:"provisioning_time_seconds"`
		RegistrationTimeSeconds   float64 `json:"registration_time_seconds"`
		PodReadinessTimeSeconds   float64 `json:"pod_readiness_time_seconds"`
		DeregistrationTimeSeconds float64 `json:"deregistration_time_seconds"`
		TerminationTimeSeconds    float64 `json:"termination_time_seconds"`
		TotalScaleUpTimeSeconds   float64 `json:"total_scale_up_time_seconds"`
		TotalScaleDownTimeSeconds float64 `json:"total_scale_down_time_seconds"`
	} `json:"results"`
}

// SaveBenchmarkReport saves the benchmark results to a JSON file.
func SaveBenchmarkReport(
	outputPath string,
	cfg *config.Config,
	autoscalerType string,
	result *BenchmarkResult,
) error {
	// Create the report
	report := &BenchmarkReport{
		Timestamp:  time.Now(),
		AutoScaler: autoscalerType,
	}

	// Populate config
	report.Config.Namespace = cfg.Namespace
	report.Config.Replicas = cfg.Replicas
	report.Config.CPURequest = cfg.CPURequest
	report.Config.ContainerImage = cfg.ContainerImage

	// Populate results
	report.Results.ProvisioningTimeSeconds = result.ProvisioningTime.Seconds()
	report.Results.RegistrationTimeSeconds = result.RegistrationTime.Seconds()
	report.Results.PodReadinessTimeSeconds = result.PodReadinessTime.Seconds()
	report.Results.DeregistrationTimeSeconds = result.DeregistrationTime.Seconds()
	report.Results.TerminationTimeSeconds = result.TerminationTime.Seconds()

	// Calculate totals
	report.Results.TotalScaleUpTimeSeconds =
		result.ProvisioningTime.Seconds() +
			result.RegistrationTime.Seconds() +
			result.PodReadinessTime.Seconds()

	report.Results.TotalScaleDownTimeSeconds =
		result.DeregistrationTime.Seconds() +
			result.TerminationTime.Seconds()

	// Create output file
	if outputPath == "" {
		timestamp := time.Now().Format("20060102-150405")
		outputPath = fmt.Sprintf("benchmark-report-%s-%s.json", autoscalerType, timestamp)
	}

	// Serialize to JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize report to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write report to file: %w", err)
	}

	fmt.Printf("Benchmark report saved to: %s\n", outputPath)
	return nil
}

// DisplaySummary prints a summary of the benchmark results to stdout.
func DisplaySummary(autoscalerType string, result *BenchmarkResult) {
	util.PrintSummary(
		result.ProvisioningTime,
		result.RegistrationTime,
		result.PodReadinessTime,
		result.DeregistrationTime,
		result.TerminationTime,
	)

	// Calculate and display totals
	totalScaleUp := result.ProvisioningTime + result.RegistrationTime + result.PodReadinessTime
	totalScaleDown := result.DeregistrationTime + result.TerminationTime

	fmt.Printf("\n%s%sScaling Summary for %s%s\n", config.ColorBold, config.ColorCyan, autoscalerType, config.ColorReset)
	fmt.Printf("%s%s--------------------------------------------%s\n", config.ColorBold, config.ColorYellow, config.ColorReset)
	fmt.Printf("%sTotal Scale-Up Time:   %s%.2f seconds%s\n", config.ColorBold+config.ColorGreen, config.ColorReset, totalScaleUp.Seconds(), config.ColorReset)
	fmt.Printf("%sTotal Scale-Down Time: %s%.2f seconds%s\n", config.ColorBold+config.ColorRed, config.ColorReset, totalScaleDown.Seconds(), config.ColorReset)
	fmt.Printf("%s%s--------------------------------------------%s\n\n", config.ColorBold, config.ColorYellow, config.ColorReset)
}
