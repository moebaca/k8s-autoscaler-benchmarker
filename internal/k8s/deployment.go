// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package k8s provides functionality to orchestrate the deployment, scaling,
// and monitoring of Kubernetes resources as part of the k8s-autoscaler-benchmarker project.
package k8s

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/aws"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/utilities"
)

// CheckNodeGroupEmpty checks whether a specified node group within a Kubernetes cluster
// has any nodes. It returns true if the node group is empty, and false otherwise.
// This check is useful for ensuring that a node group can be safely manipulated without
// affecting any existing nodes within it.
func CheckNodeGroupEmpty(clientset kubernetes.Interface, nodeGroupName string) bool {
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("eks.amazonaws.com/nodegroup=%s", nodeGroupName),
	})
	if err != nil {
		log.Fatalf("Failed to list nodes: %v", err)
	}

	return len(nodes.Items) == 0
}

// ScaleDeployment updates the number of replicas for a specified deployment within a given namespace.
// It first retrieves the current deployment settings, then updates the replica count based on the input parameter.
// The function logs whether the deployment was scaled up, down, or remained unchanged.
func ScaleDeployment(clientset kubernetes.Interface, deploymentName, namespace string, replicas int) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	deployment, err := deploymentsClient.Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get deployment: %v", err)
	}

	currentReplicas := deployment.Spec.Replicas

	deployment.Spec.Replicas = utilities.Int32Ptr(int32(replicas))
	_, err = deploymentsClient.Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("Failed to update deployment: %v", err)
	}

	if int32(replicas) > *currentReplicas {
		fmt.Printf("Deployment '%s' scaled up to %d replicas successfully.\n", deploymentName, replicas)
	} else if int32(replicas) < *currentReplicas {
		fmt.Printf("Deployment '%s' scaled down to %d replicas successfully.\n", deploymentName, replicas)
	} else {
		fmt.Printf("Deployment '%s' already has %d replicas. No scaling performed.\n", deploymentName, replicas)
	}
}

// MonitorProvisioning tracks the provisioning status of EC2 instances by filtering with tag key and value.
// It prompts the user for action if provisioning exceeds the predefined timeout.
// The function logs the provisioning status, including launched instances, until all required instances are in a 'Pending' state or a user intervention occurs.
func MonitorProvisioning(clientset kubernetes.Interface, ec2Svc *ec2.EC2, tagKey, tagValue, deploymentName, namespace string) time.Duration {
	fmt.Println("Monitoring provisioning...")
	startTime := time.Now()
	var instanceDetails []string
	reader := bufio.NewReader(os.Stdin)

	timeout := 60 * time.Second

	for {
		time.Sleep(1 * time.Second)
		if time.Since(startTime) >= timeout {
			for {
				fmt.Println("Provisioning timeout exceeded. There may be an issue (check pod for errors). Do you want to continue waiting to troubleshoot issue? [yes/no]: ")
				answer, err := reader.ReadString('\n')
				if err != nil {
					log.Fatalf("Failed to read input: %v", err)
				}
				answer = strings.TrimSpace(answer)
				if answer == "no" {
					fmt.Println("Exiting due to user input.")

					if deploymentName == "" {
						DeleteDeployment(clientset, deploymentName, namespace)
					}
					os.Exit(1)
				} else if answer == "yes" {
					startTime = time.Now()
					fmt.Println("Please input 'no' at next timeout instead of force closing so that cleanup steps can be run by the program...")
					break
				} else {
					fmt.Println("Invalid input. Please enter 'yes' or 'no'.")
				}
			}
		}

		instances, err := aws.GetEC2Instances(ec2Svc, "tag:"+tagKey, tagValue)
		if err != nil {
			log.Fatalf("Error retrieving EC2 instances: %v", err)
		}

		if len(instances) > 0 && *instances[0].State.Name == ec2.InstanceStateNamePending {
			for _, instance := range instances {
				detail := fmt.Sprintf("%s (%s)", *instance.InstanceId, *instance.PrivateDnsName)
				instanceDetails = append(instanceDetails, detail)
			}
			fmt.Println("Instances launched:", strings.Join(instanceDetails, ", "))
			return time.Since(startTime)
		}
	}
}

// WaitForPodsReady waits until all pods in a deployment reach a 'Ready' state.
// It periodically checks the deployment's status and logs the current count of ready pods against the total number of replicas until all pods are ready.
func WaitForPodsReady(clientset kubernetes.Interface, deploymentName, namespace string, replicas int) time.Duration {
	fmt.Println("Waiting for pods to become ready...")
	startTime := time.Now()
	logTicker := time.NewTicker(15 * time.Second)
	defer logTicker.Stop()

	for {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Failed to get updated deployment: %v", err)
		}

		if deployment.Status.ReadyReplicas == int32(replicas) {
			fmt.Println("All pods are ready.")
			break
		}

		select {
		case <-logTicker.C:
			fmt.Printf("Waiting... %d/%d pods are ready.\n", deployment.Status.ReadyReplicas, replicas)
		default:
			time.Sleep(1 * time.Second)
		}
	}

	return time.Since(startTime)
}

// MonitorNodeDeregistration observes the deregistration of nodes from the Kubernetes API based on label selectors.
// It continuously checks and logs the registered nodes until none are left, signaling complete deregistration.
func MonitorNodeDeregistration(clientset *kubernetes.Clientset, nodeSelectorKey, nodeSelectorValue string, deregChan chan<- time.Duration) {
	startTime := time.Now()
	logTicker := time.NewTicker(15 * time.Second)
	defer logTicker.Stop()

	fmt.Println("Monitoring node deregistration from k8s API...")

	for {
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", nodeSelectorKey, nodeSelectorValue),
		})
		if err != nil {
			log.Fatalf("Failed to list nodes: %v", err)
		}

		if len(nodes.Items) == 0 {
			fmt.Println("All nodes have been deregistered from k8s API.")
			deregChan <- time.Since(startTime)
			return
		}

		select {
		case <-logTicker.C:
			var nodeNames []string
			for _, node := range nodes.Items {
				nodeNames = append(nodeNames, node.Name)
			}
			fmt.Printf("Nodes still registered to the cluster: %s\n", strings.Join(nodeNames, ", "))
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

// MonitorNodeTermination keeps an eye on the termination process of EC2 instances, ensuring all tagged instances are terminated.
// It logs the status of running instances and waits until no tagged instances are left running.
func MonitorNodeTermination(ec2Svc *ec2.EC2, tagKey, tagValue string, termChan chan<- time.Duration) {
	fmt.Println("Monitoring EC2 instance termination...")
	startTime := time.Now()
	logTicker := time.NewTicker(15 * time.Second)
	defer logTicker.Stop()

	for {
		instances, err := aws.GetEC2Instances(ec2Svc, "tag:"+tagKey, tagValue)
		if err != nil {
			log.Printf("Monitoring error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(instances) == 0 {
			fmt.Println("All EC2 instances have been terminated.")
			termChan <- time.Since(startTime)
			return
		}

		select {
		case <-logTicker.C:
			var instanceDetails []string
			for _, instance := range instances {
				detail := fmt.Sprintf("%s (%s)", *instance.InstanceId, *instance.PrivateDnsName)
				instanceDetails = append(instanceDetails, detail)
			}
			if len(instanceDetails) > 0 {
				fmt.Println("EC2 instances still running:", strings.Join(instanceDetails, ", "))
			}
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

// GenerateDeployment creates a new Kubernetes deployment using specified parameters, including deployment name, namespace, and container configuration.
// It sets up tolerations and node selectors for the deployment and logs the creation status.
func GenerateDeployment(clientset kubernetes.Interface, deploymentName, namespace, containerName, containerImage, cpuRequest, tolerationKey, tolerationValue, nodeSelectorKey, nodeSelectorValue string, replicas int) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	// Ensure deploymentName is not empty
	if deploymentName == "" {
		log.Fatal("Deployment name must not be empty")
	}

	// Define labels to be used by both the selector and the pod template
	labels := map[string]string{
		"app": deploymentName,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: utilities.Int32Ptr(int32(replicas)),
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

	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Failed to create deployment: %v", err)
	}
	fmt.Printf("Created deployment %q in namespace %q.\n", result.GetObjectMeta().GetName(), namespace)
}

// DeleteDeployment removes a specified deployment from a given namespace.
// It ensures the deployment is deleted according to the specified deletion policy and logs the deletion status.
func DeleteDeployment(clientset kubernetes.Interface, deploymentName, namespace string) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	fmt.Printf("Deleting deployment %q in namespace %q...\n", deploymentName, namespace)
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.Background(), deploymentName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		log.Fatalf("Failed to delete deployment: %v", err)
	}
	fmt.Printf("Deleted deployment %q.\n", deploymentName)
}
