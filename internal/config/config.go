// Package config provides configuration for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package config

import (
	"flag"
	"fmt"
	"time"
)

// Config holds the application configuration.
type Config struct {
	// Kubernetes configuration
	KubeconfigPath string
	Namespace      string

	// AWS configuration
	AWSProfile string

	// Deployment configuration
	DeploymentName    string
	Replicas          int
	ContainerName     string
	ContainerImage    string
	CPURequest        string
	TolerationsKey    string
	TolerationsValue  string
	NodeSelectorKey   string
	NodeSelectorValue string

	// Autoscaler configuration
	NodepoolTag string // For Karpenter
	NodeGroup   string // For Cluster Autoscaler

	// Runtime configuration
	Timeouts TimeoutConfig
}

// TimeoutConfig holds timeout configurations for various operations.
type TimeoutConfig struct {
	ProvisioningTimeout   time.Duration
	RegistrationTimeout   time.Duration
	ReadinessTimeout      time.Duration
	DeregistrationTimeout time.Duration
	TerminationTimeout    time.Duration
}

// LoadConfig parses command line flags and returns a populated Config.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Timeouts: TimeoutConfig{
			ProvisioningTimeout:   10 * time.Minute,
			RegistrationTimeout:   10 * time.Minute,
			ReadinessTimeout:      5 * time.Minute,
			DeregistrationTimeout: 10 * time.Minute,
			TerminationTimeout:    10 * time.Minute,
		},
	}

	// Parse command line flags
	flag.StringVar(&cfg.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file")
	flag.StringVar(&cfg.AWSProfile, "aws-profile", "default", "AWS profile to use")
	flag.StringVar(&cfg.DeploymentName, "deployment", "", "Deployment name to benchmark")
	flag.StringVar(&cfg.Namespace, "namespace", "default", "Deployment namespace")
	flag.IntVar(&cfg.Replicas, "replicas", 1, "Number of replicas")
	flag.StringVar(&cfg.NodepoolTag, "nodepool", "", "Karpenter node pool tag value")
	flag.StringVar(&cfg.NodeGroup, "node-group", "", "ASG node group name")
	flag.StringVar(&cfg.ContainerName, "container-name", "inflate", "Name for generated container")
	flag.StringVar(&cfg.ContainerImage, "container-image", "public.ecr.aws/eks-distro/kubernetes/pause:3.7", "Container image")
	flag.StringVar(&cfg.CPURequest, "cpu-request", "1", "CPU request for container")
	flag.StringVar(&cfg.TolerationsKey, "toleration-key", "eks.autify.com/k8s-autoscaler-benchmarker", "Toleration key")
	flag.StringVar(&cfg.TolerationsValue, "toleration-value", "", "Toleration value")
	flag.StringVar(&cfg.NodeSelectorKey, "node-selector-key", "eks.autify.com/k8s-autoscaler-benchmarker", "Node selector key")
	flag.StringVar(&cfg.NodeSelectorValue, "node-selector-value", "true", "Node selector value")

	// Parse additional timeout flags
	var provTimeout, regTimeout, readyTimeout, deregTimeout, termTimeout int
	flag.IntVar(&provTimeout, "provisioning-timeout", 10, "Timeout for instance provisioning in minutes")
	flag.IntVar(&regTimeout, "registration-timeout", 10, "Timeout for node registration in minutes")
	flag.IntVar(&readyTimeout, "readiness-timeout", 5, "Timeout for pod readiness in minutes")
	flag.IntVar(&deregTimeout, "deregistration-timeout", 10, "Timeout for node deregistration in minutes")
	flag.IntVar(&termTimeout, "termination-timeout", 10, "Timeout for instance termination in minutes")

	flag.Parse()

	// Convert timeout minutes to time.Duration
	cfg.Timeouts.ProvisioningTimeout = time.Duration(provTimeout) * time.Minute
	cfg.Timeouts.RegistrationTimeout = time.Duration(regTimeout) * time.Minute
	cfg.Timeouts.ReadinessTimeout = time.Duration(readyTimeout) * time.Minute
	cfg.Timeouts.DeregistrationTimeout = time.Duration(deregTimeout) * time.Minute
	cfg.Timeouts.TerminationTimeout = time.Duration(termTimeout) * time.Minute

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateConfig checks if the configuration is valid.
func validateConfig(cfg *Config) error {
	// Ensure either Karpenter or Cluster Autoscaler is specified, not both
	if (cfg.NodepoolTag != "" && cfg.NodeGroup != "") || (cfg.NodepoolTag == "" && cfg.NodeGroup == "") {
		return fmt.Errorf("specify either --nodepool for Karpenter or --node-group for Cluster Autoscaler, not both")
	}

	return nil
}
