// Package config provides configuration for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package config

import "time"

// Runtime constants
var (
	// ProgramStartTime records when the program started, used to identify resources created by this run.
	ProgramStartTime = time.Now()
)

// Output formatting constants
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

// Karpenter constants
const (
	KarpenterNodePoolLabelKey = "karpenter.sh/nodepool"
)

// Cluster Autoscaler constants
const (
	ClusterAutoscalerNodeGroupLabelKey = "eks.amazonaws.com/nodegroup"
	EksNodeGroupTagKey                 = "eks:nodegroup-name"
)

// EC2 instance states
const (
	InstanceStatePending     = "pending"
	InstanceStateRunning     = "running"
	InstanceStateTerminating = "terminating"
	InstanceStateTerminated  = "terminated"
)

// Polling intervals
const (
	NodeStatusPollInterval   = 5 * time.Second
	PodStatusPollInterval    = 1 * time.Second
	StatusLogInterval        = 15 * time.Second
	PodReadinessLogInterval  = 20 * time.Second
	EC2InstanceCheckInterval = 5 * time.Second
)

// Error retry configuration
const (
	MaxRetries       = 5
	BaseBackoffDelay = 1 * time.Second
)
