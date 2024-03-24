// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package main is the entry point for the k8s-autoscaler-benchmarker tool.
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/k8s"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/utilities"
)

var (
	kubeconfigPath    string
	awsProfile        string
	deploymentName    string
	namespace         string
	replicas          int
	nodepoolTag       string
	nodeGroup         string
	containerName     string
	containerImage    string
	cpuRequest        string
	tolerationKey     string
	tolerationValue   string
	nodeSelectorKey   string
	nodeSelectorValue string
)

func init() {
	flag.StringVar(&kubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
	flag.StringVar(&awsProfile, "aws-profile", "default", "The AWS profile to use.")
	flag.StringVar(&deploymentName, "deployment", "", "The deployment name to benchmark.")
	flag.StringVar(&namespace, "namespace", "default", "The namespace of the deployment.")
	flag.IntVar(&replicas, "replicas", 1, "The number of replicas to scale the deployment to.")
	flag.StringVar(&nodepoolTag, "nodepool", "", "The Karpenter node pool tag value to monitor.")
	flag.StringVar(&nodeGroup, "node-group", "", "The ASG node group name to monitor.")
	flag.StringVar(&containerName, "container-name", "inflate", "The name of the generated deployment and container if an existing deployment isn't supplied.")
	flag.StringVar(&containerImage, "container-image", "public.ecr.aws/eks-distro/kubernetes/pause:3.7", "The image of the container in the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&cpuRequest, "cpu-request", "1", "The CPU request for the container in the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&tolerationKey, "toleration-key", "eks.autify.com/k8s-autoscaler-benchmarker", "The toleration key for the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&tolerationValue, "toleration-value", "", "The toleration value for the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&nodeSelectorKey, "node-selector-key", "eks.autify.com/k8s-autoscaler-benchmarker", "The node selector key for the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&nodeSelectorValue, "node-selector-value", "true", "The node selector value for the generated deployment if an existing deployment isn't supplied.")
	flag.Parse()
}

// Command k8s-autoscaler-benchmarker orchestrates the setup, execution, and teardown
// of a Kubernetes deployment to benchmark either Cluster Autoscaler or Karpenter autoscaling functionalities.
// It determines the autoscaler type based on input flags, creates or uses an existing deployment, scales it,
// and then monitors the provisioning of nodes and readiness of pods.
// It concludes by scaling down the deployment and monitoring node deregistration and EC2 instance termination,
// before printing out a summary of the benchmark results to stdout.
func main() {
	var autoscalerType, tagKey, tagValue string

	kubeconfig := kubeconfigPath
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create kubernetes clientset: %v", err)
	}

	awsSessionOpts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Profile:           awsProfile,
	}
	awsSession := session.Must(session.NewSessionWithOptions(awsSessionOpts))
	ec2Svc := ec2.New(awsSession)

	_, err = ec2Svc.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		log.Fatalf("Failed to test AWS profile '%s': %v. Ensure the AWS profile is configured correctly.", awsProfile, err)
	}

	if nodepoolTag != "" && nodeGroup == "" {
		autoscalerType = "Karpenter"
		tagKey = "karpenter.sh/nodepool"
		tagValue = nodepoolTag
	} else if nodeGroup != "" && nodepoolTag == "" {
		autoscalerType = "Cluster Autoscaler"
		tagKey = "eks:nodegroup-name"
		tagValue = nodeGroup

		if !k8s.CheckNodeGroupEmpty(clientset, nodeGroup) {
			log.Fatalf("Node group '%s' is not empty. Please ensure desired capacity is set to 0 before running the benchmark.", nodeGroup)
		}
	} else {
		log.Fatal("Specify either --nodepool for Karpenter or --node-group for Cluster Autoscaler, not both.")
	}

	fmt.Printf("Testing with %s...\n", autoscalerType)

	if deploymentName == "" {
		deploymentName = containerName
		fmt.Printf("No existing deployment name supplied, using '%s' for new deployment.\n", deploymentName)
		k8s.GenerateDeployment(clientset, deploymentName, namespace, containerName, containerImage, cpuRequest, tolerationKey, tolerationValue, nodeSelectorKey, nodeSelectorValue, replicas)
		defer k8s.DeleteDeployment(clientset, deploymentName, namespace)
	} else {
		fmt.Printf("Using user supplied deployment named '%s' in the namespace '%s'.\n", deploymentName, namespace)
		k8s.ScaleDeployment(clientset, deploymentName, namespace, replicas)
	}

	provisioningTime := k8s.MonitorProvisioning(clientset, ec2Svc, tagKey, tagValue, deploymentName, namespace)
	podReadinessTime := k8s.WaitForPodsReady(clientset, deploymentName, namespace, replicas)

	k8s.ScaleDeployment(clientset, deploymentName, namespace, 0)

	deregChan := make(chan time.Duration)
	termChan := make(chan time.Duration)
	go k8s.MonitorNodeDeregistration(clientset, nodeSelectorKey, nodeSelectorValue, deregChan)
	go k8s.MonitorNodeTermination(ec2Svc, tagKey, tagValue, termChan)

	nodeDeregistrationTime := <-deregChan
	terminationTime := <-termChan

	utilities.PrintSummary(provisioningTime, podReadinessTime, nodeDeregistrationTime, terminationTime)
}
