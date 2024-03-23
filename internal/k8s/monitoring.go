// Package k8s provides Kubernetes interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MonitorNodeRegistration waits for nodes to register and become ready.
func MonitorNodeRegistration(ctx context.Context, clientset kubernetes.Interface, labelSelector string, expectedNodeCount int) (time.Duration, error) {
	fmt.Println("Monitoring instance registration to k8s API...")
	startTime := time.Now()

	ticker := time.NewTicker(config.NodeStatusPollInterval)
	defer ticker.Stop()

	logTicker := time.NewTicker(config.StatusLogInterval)
	defer logTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return time.Since(startTime), ctx.Err()

		case <-logTicker.C:
			// Periodically log the status
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				fmt.Printf("Warning: failed to list nodes: %v\n", err)
				continue
			}

			readyCount := countReadyNodes(nodes.Items)
			fmt.Printf("Node registration status: %d/%d nodes ready\n", readyCount, expectedNodeCount)

		case <-ticker.C:
			// Check if nodes are ready
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to list nodes with selector %s: %w", labelSelector, err)
			}

			readyCount := countReadyNodes(nodes.Items)
			if readyCount >= expectedNodeCount {
				fmt.Printf("%d nodes registered and ready in k8s API.\n", readyCount)
				return time.Since(startTime), nil
			}
		}
	}
}

// countReadyNodes counts nodes that are in Ready condition.
func countReadyNodes(nodes []corev1.Node) int {
	readyCount := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyCount++
				break
			}
		}
	}
	return readyCount
}

// WaitForPodsReady waits for all pods in a deployment to be ready.
func WaitForPodsReady(ctx context.Context, clientset kubernetes.Interface, deploymentName, namespace string, replicas int) (time.Duration, error) {
	fmt.Println("Waiting for pods to become ready...")
	startTime := time.Now()

	ticker := time.NewTicker(config.PodStatusPollInterval)
	defer ticker.Stop()

	logTicker := time.NewTicker(config.PodReadinessLogInterval)
	defer logTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return time.Since(startTime), ctx.Err()

		case <-logTicker.C:
			// Log the current pod status
			deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				fmt.Printf("Warning: failed to get deployment status: %v\n", err)
				continue
			}

			fmt.Printf("Pod readiness: %d/%d pods ready\n", deployment.Status.ReadyReplicas, replicas)

		case <-ticker.C:
			// Check if all pods are ready
			deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				return 0, fmt.Errorf("failed to get deployment status: %w", err)
			}

			if deployment.Status.ReadyReplicas == int32(replicas) {
				fmt.Println("All pods are ready.")
				return time.Since(startTime), nil
			}
		}
	}
}

// MonitorNodeDeregistration monitors nodes until they're deregistered from K8s.
func MonitorNodeDeregistration(ctx context.Context, clientset kubernetes.Interface, nodeSelectorKey, nodeSelectorValue string) (time.Duration, error) {
	fmt.Println("Monitoring node deregistration from k8s API...")
	startTime := time.Now()

	ticker := time.NewTicker(config.NodeStatusPollInterval)
	defer ticker.Stop()

	logTicker := time.NewTicker(config.StatusLogInterval)
	defer logTicker.Stop()

	labelSelector := fmt.Sprintf("%s=%s", nodeSelectorKey, nodeSelectorValue)

	for {
		select {
		case <-ctx.Done():
			return time.Since(startTime), ctx.Err()

		case <-logTicker.C:
			// Log the current node status
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				fmt.Printf("Warning: failed to list nodes: %v\n", err)
				continue
			}

			if len(nodes.Items) > 0 {
				nodeNames := make([]string, 0, len(nodes.Items))
				for _, node := range nodes.Items {
					nodeNames = append(nodeNames, node.Name)
				}
				fmt.Printf("Nodes still registered: %s\n", strings.Join(nodeNames, ", "))
			}

		case <-ticker.C:
			// Check if all nodes are deregistered
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to list nodes: %w", err)
			}

			if len(nodes.Items) == 0 {
				fmt.Println("All nodes have been deregistered from k8s API.")
				return time.Since(startTime), nil
			}
		}
	}
}
