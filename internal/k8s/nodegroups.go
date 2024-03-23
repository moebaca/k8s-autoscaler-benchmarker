// Package k8s provides Kubernetes interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CheckNodeGroupEmpty verifies if a node group is empty.
// Returns true if no nodes with the given label selector exist, false otherwise.
func CheckNodeGroupEmpty(ctx context.Context, clientset kubernetes.Interface, labelSelector string) (bool, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	return len(nodes.Items) == 0, nil
}

// VerifyAutoscalerReady checks if the selected autoscaler is properly configured.
// For Cluster Autoscaler, it verifies the node group is empty.
func VerifyAutoscalerReady(ctx context.Context, clientset kubernetes.Interface, nodepoolTag, nodeGroup string) error {
	// Only need to check if using Cluster Autoscaler
	if nodeGroup == "" {
		return nil
	}

	// Verify node group is empty for Cluster Autoscaler
	labelSelector, err := GetLabelSelectorForAutoscaler("", nodeGroup)
	if err != nil {
		return err
	}

	isEmpty, err := CheckNodeGroupEmpty(ctx, clientset, labelSelector)
	if err != nil {
		return err
	}

	if !isEmpty {
		return fmt.Errorf("node group '%s' is not empty; set desired capacity to 0 before running benchmark", nodeGroup)
	}

	return nil
}

// GetNodeNames extracts node names from a list of node IDs.
func GetNodeNames(ctx context.Context, clientset kubernetes.Interface, labelSelector string) ([]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodeNames := make([]string, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}

	return nodeNames, nil
}
