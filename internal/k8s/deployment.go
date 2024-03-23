// Package k8s provides Kubernetes interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package k8s

import (
	"context"
	"fmt"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateDeployment creates a new Kubernetes deployment.
func CreateDeployment(
	ctx context.Context,
	clientset kubernetes.Interface,
	deploymentName,
	namespace,
	containerName,
	containerImage,
	cpuRequest,
	tolerationKey,
	tolerationValue,
	nodeSelectorKey,
	nodeSelectorValue string,
	replicas int,
) error {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	if deploymentName == "" {
		return fmt.Errorf("deployment name must not be empty")
	}

	// Define labels for the deployment
	labels := map[string]string{
		"app": deploymentName,
	}

	// Create deployment spec
	deployment := createDeploymentSpec(
		deploymentName,
		containerName,
		containerImage,
		cpuRequest,
		tolerationKey,
		tolerationValue,
		nodeSelectorKey,
		nodeSelectorValue,
		labels,
		replicas,
	)

	// Create the deployment
	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	fmt.Printf("Created deployment %q in namespace %q.\n", result.GetObjectMeta().GetName(), namespace)
	return nil
}

// createDeploymentSpec constructs a deployment specification.
func createDeploymentSpec(
	deploymentName,
	containerName,
	containerImage,
	cpuRequest,
	tolerationKey,
	tolerationValue,
	nodeSelectorKey,
	nodeSelectorValue string,
	labels map[string]string,
	replicas int,
) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.Int32Ptr(int32(replicas)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: containerImage,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse(cpuRequest),
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      tolerationKey,
							Operator: corev1.TolerationOpEqual,
							Value:    tolerationValue,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      nodeSelectorKey,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{nodeSelectorValue},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// ScaleDeployment updates the replicas for a deployment.
func ScaleDeployment(ctx context.Context, clientset kubernetes.Interface, deploymentName, namespace string, replicas int) error {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	// Get current deployment
	deployment, err := deploymentsClient.Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	currentReplicas := deployment.Spec.Replicas

	// Update replica count
	deployment.Spec.Replicas = util.Int32Ptr(int32(replicas))
	_, err = deploymentsClient.Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	// Log scaling action
	if int32(replicas) > *currentReplicas {
		fmt.Printf("Deployment '%s' scaled up to %d replicas successfully.\n", deploymentName, replicas)
	} else if int32(replicas) < *currentReplicas {
		fmt.Printf("Deployment '%s' scaled down to %d replicas successfully.\n", deploymentName, replicas)
	} else {
		fmt.Printf("Deployment '%s' already has %d replicas. No scaling performed.\n", deploymentName, replicas)
	}

	return nil
}

// DeleteDeployment removes a deployment.
func DeleteDeployment(ctx context.Context, clientset kubernetes.Interface, deploymentName, namespace string) error {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	fmt.Printf("Deleting deployment %q in namespace %q...\n", deploymentName, namespace)

	// Use foreground deletion policy to ensure dependent objects are deleted
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(ctx, deploymentName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	fmt.Printf("Deleted deployment %q.\n", deploymentName)
	return nil
}

// SetupDeployment creates a new deployment or scales an existing one.
func SetupDeployment(
	ctx context.Context,
	clientset kubernetes.Interface,
	cfg *DeploymentConfig,
) error {
	if cfg.DeploymentName == "" {
		// Create new deployment
		fmt.Printf("Creating new deployment with name %q\n", cfg.ContainerName)
		return CreateDeployment(
			ctx,
			clientset,
			cfg.ContainerName, // Use container name as deployment name
			cfg.Namespace,
			cfg.ContainerName,
			cfg.ContainerImage,
			cfg.CPURequest,
			cfg.TolerationsKey,
			cfg.TolerationsValue,
			cfg.NodeSelectorKey,
			cfg.NodeSelectorValue,
			cfg.Replicas,
		)
	}

	// Scale existing deployment
	fmt.Printf("Using existing deployment %q\n", cfg.DeploymentName)
	return ScaleDeployment(ctx, clientset, cfg.DeploymentName, cfg.Namespace, cfg.Replicas)
}

// DeploymentConfig holds configuration for deployments.
type DeploymentConfig struct {
	DeploymentName    string
	Namespace         string
	ContainerName     string
	ContainerImage    string
	CPURequest        string
	TolerationsKey    string
	TolerationsValue  string
	NodeSelectorKey   string
	NodeSelectorValue string
	Replicas          int
}
