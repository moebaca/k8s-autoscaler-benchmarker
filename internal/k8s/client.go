// Package k8s provides Kubernetes interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient creates a new Kubernetes client from the provided kubeconfig path.
// If kubeconfigPath is empty, it uses the default kubeconfig location.
func NewClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	// If no kubeconfig path is provided, use the default
	configPath := kubeconfigPath
	if configPath == "" {
		configPath = clientcmd.RecommendedHomeFile
	}

	// Build the config from the specified kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return clientset, nil
}

// GetLabelSelectorForAutoscaler returns the appropriate label selector based on the autoscaler type.
// For Karpenter, it uses the nodepool tag; for Cluster Autoscaler, it uses the node group name.
func GetLabelSelectorForAutoscaler(nodepoolTag, nodeGroup string) (string, error) {
	if nodepoolTag != "" && nodeGroup == "" {
		// Karpenter
		return fmt.Sprintf("karpenter.sh/nodepool=%s", nodepoolTag), nil
	} else if nodeGroup != "" && nodepoolTag == "" {
		// Cluster Autoscaler
		return fmt.Sprintf("eks.amazonaws.com/nodegroup=%s", nodeGroup), nil
	}

	return "", fmt.Errorf("either nodepool tag or node group must be specified, not both or neither")
}

// GetTagKeyForAutoscaler returns the appropriate EC2 tag key based on the autoscaler type.
func GetTagKeyForAutoscaler(nodepoolTag, nodeGroup string) (string, error) {
	if nodepoolTag != "" && nodeGroup == "" {
		// Karpenter
		return "karpenter.sh/nodepool", nil
	} else if nodeGroup != "" && nodepoolTag == "" {
		// Cluster Autoscaler
		return "eks:nodegroup-name", nil
	}

	return "", fmt.Errorf("either nodepool tag or node group must be specified, not both or neither")
}

// GetAutoscalerType returns a string describing which autoscaler is being used.
func GetAutoscalerType(nodepoolTag, nodeGroup string) (string, error) {
	if nodepoolTag != "" && nodeGroup == "" {
		return "Karpenter", nil
	} else if nodeGroup != "" && nodepoolTag == "" {
		return "Cluster Autoscaler", nil
	}

	return "", fmt.Errorf("either nodepool tag or node group must be specified, not both or neither")
}

// GetTagValueForAutoscaler returns the appropriate tag value based on the autoscaler type.
func GetTagValueForAutoscaler(nodepoolTag, nodeGroup string) (string, error) {
	if nodepoolTag != "" && nodeGroup == "" {
		return nodepoolTag, nil
	} else if nodeGroup != "" && nodepoolTag == "" {
		return nodeGroup, nil
	}

	return "", fmt.Errorf("either nodepool tag or node group must be specified, not both or neither")
}
