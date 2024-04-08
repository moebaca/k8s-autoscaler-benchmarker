// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package main is the entry point for the k8s-autoscaler-benchmarker tool.
package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/aws"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/k8s"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/utilities"
)

type Config struct {
	kubeconfigPath, awsProfile, deploymentName, namespace string
	replicas                                              int
	nodepoolTag, nodeGroup, containerName, containerImage string
	cpuRequest, tolerationKey, tolerationValue            string
	nodeSelectorKey, nodeSelectorValue                    string
}

// parseFlags parses the command line flags and returns a Config struct populated with the command line arguments.
// This function supports a variety of flags for configuring the Kubernetes client, AWS session, deployment parameters, and autoscaler settings.
func parseFlags() Config {
	var config Config

	flag.StringVar(&config.kubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
	flag.StringVar(&config.awsProfile, "aws-profile", "default", "The AWS profile to use.")
	flag.StringVar(&config.deploymentName, "deployment", "", "The deployment name to benchmark.")
	flag.StringVar(&config.namespace, "namespace", "default", "The namespace of the deployment.")
	flag.IntVar(&config.replicas, "replicas", 1, "The number of replicas to scale the deployment to.")
	flag.StringVar(&config.nodepoolTag, "nodepool", "", "The Karpenter node pool tag value to monitor.")
	flag.StringVar(&config.nodeGroup, "node-group", "", "The ASG node group name to monitor.")
	flag.StringVar(&config.containerName, "container-name", "inflate", "The name of the generated deployment and container if an existing deployment isn't supplied.")
	flag.StringVar(&config.containerImage, "container-image", "public.ecr.aws/eks-distro/kubernetes/pause:3.7", "The image of the container in the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&config.cpuRequest, "cpu-request", "1", "The CPU request for the container in the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&config.tolerationKey, "toleration-key", "eks.autify.com/k8s-autoscaler-benchmarker", "The toleration key for the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&config.tolerationValue, "toleration-value", "", "The toleration value for the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&config.nodeSelectorKey, "node-selector-key", "eks.autify.com/k8s-autoscaler-benchmarker", "The node selector key for the generated deployment if an existing deployment isn't supplied.")
	flag.StringVar(&config.nodeSelectorValue, "node-selector-value", "true", "The node selector value for the generated deployment if an existing deployment isn't supplied.")
	flag.Parse()

	return config
}

// initializeClients initializes and returns Kubernetes and AWS EC2 clients using the provided configuration.
// It uses the kubeconfigPath for the Kubernetes client and the awsProfile for the AWS session.
// This function logs a fatal error and exits the program if either client cannot be initialized successfully.
func initializeClients(kubeconfigPath, awsProfile string) (*kubernetes.Clientset, *ec2.EC2) {
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
	if _, err := ec2Svc.DescribeRegions(&ec2.DescribeRegionsInput{}); err != nil {
		log.Fatalf("Failed to test AWS profile '%s': %v. Ensure the AWS profile is configured correctly.", awsProfile, err)
	}

	return clientset, ec2Svc
}

// determineAutoscalerType determines the type of autoscaler to be benchmarked based on the provided configuration.
// It returns the autoscaler type ("Karpenter" or "Cluster Autoscaler"), along with the tag key and value to be used for monitoring.
// This function checks the configuration to ensure that only one autoscaler type is specified and logs a fatal error if the configuration is invalid.
func determineAutoscalerType(config Config, clientset *kubernetes.Clientset) (string, string, string) {
	var autoscalerType, tagKey, tagValue, labelSelector string

	if config.nodepoolTag != "" && config.nodeGroup == "" {
		autoscalerType = "Karpenter"
		tagKey = "karpenter.sh/nodepool"
		tagValue = config.nodepoolTag
		labelSelector = fmt.Sprintf("karpenter.sh/nodepool=%s", config.nodepoolTag)
	} else if config.nodeGroup != "" && config.nodepoolTag == "" {
		autoscalerType = "Cluster Autoscaler"
		tagKey = "eks:nodegroup-name"
		tagValue = config.nodeGroup
		labelSelector = fmt.Sprintf("eks.amazonaws.com/nodegroup=%s", config.nodeGroup)

		isEmpty, err := k8s.CheckNodeGroupEmpty(clientset, labelSelector)
		if err != nil {
			log.Fatalf("Error checking if node group '%s' is empty: %v", config.nodeGroup, err)
		}
		if !isEmpty {
			log.Fatalf("Node group '%s' is not empty. Please ensure desired capacity is set to 0 before running the benchmark.", config.nodeGroup)
		}
	} else {
		log.Fatal("Specify either --nodepool for Karpenter or --node-group for Cluster Autoscaler, not both.")
	}

	fmt.Printf("Testing with %s...\n", autoscalerType)

	return labelSelector, tagKey, tagValue
}

// executeBenchmark orchestrates the benchmarking process, including deployment generation/scaling, instance provisioning and readiness monitoring, pod readiness, and cleanup.
// It takes the Kubernetes and AWS EC2 clients, the configuration, the autoscaler type, and the tag key and value for monitoring.
// This function defers the deletion of the deployment if it was created during the benchmark and handles errors encountered during the monitoring stages.
func executeBenchmark(clientset *kubernetes.Clientset, ec2Svc *ec2.EC2, config Config, labelSelector, tagKey, tagValue string) {
	var instanceDeregTime, instanceTermTime time.Duration
	var wg sync.WaitGroup
	var errMsg string

	if config.deploymentName == "" {
		config.deploymentName = config.containerName
		fmt.Printf("No existing deployment name supplied, using '%s' for new deployment.\n", config.deploymentName)
		if err := k8s.GenerateDeployment(clientset, config.deploymentName, config.namespace, config.containerName, config.containerImage, config.cpuRequest, config.tolerationKey, config.tolerationValue, config.nodeSelectorKey, config.nodeSelectorValue, config.replicas); err != nil {
			log.Fatalf("Failed to generate deployment: %v", err)
		}
		defer func() {
			if err := k8s.DeleteDeployment(clientset, config.deploymentName, config.namespace); err != nil {
				log.Printf("Failed to delete deployment: %v", err)
			}
		}()
	} else {
		fmt.Printf("Using user-supplied deployment named '%s' in the namespace '%s'.\n", config.deploymentName, config.namespace)
		if err := k8s.ScaleDeployment(clientset, config.deploymentName, config.namespace, config.replicas); err != nil {
			log.Fatalf("Failed to scale up deployment: %v", err)
		}
	}

	deregChan := make(chan time.Duration)
	termChan := make(chan time.Duration)
	errChan := make(chan error, 2)

	instanceProvisioningTime, launchedInstances, err := aws.MonitorInstanceProvisioning(clientset, ec2Svc, tagKey, tagValue, config.deploymentName, config.namespace)
	if err != nil {
		errMsg = fmt.Sprintf("Error during instance provisioning: %v", err)
		cleanupAndFatal(clientset, config, errMsg)
	}

	instanceRegistrationTime, err := k8s.MonitorInstanceRegistration(clientset, labelSelector, launchedInstances)
	if err != nil {
		errMsg = fmt.Sprintf("Error during instance registration: %v", err)
		cleanupAndFatal(clientset, config, errMsg)
	}

	podReadinessTime, err := k8s.WaitForPodsReady(clientset, config.deploymentName, config.namespace, config.replicas)
	if err != nil {
		errMsg = fmt.Sprintf("Error during pod readiness: %v", err)
		cleanupAndFatal(clientset, config, errMsg)
	}

	if err := k8s.ScaleDeployment(clientset, config.deploymentName, config.namespace, 0); err != nil {
		errMsg = fmt.Sprintf("Failed to scale down deployment to 0: %v", err)
		cleanupAndFatal(clientset, config, errMsg)
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		k8s.MonitorNodeDeregistration(clientset, config.nodeSelectorKey, config.nodeSelectorValue, deregChan, errChan)
	}()
	go func() {
		defer wg.Done()
		k8s.MonitorNodeTermination(ec2Svc, tagKey, tagValue, termChan, errChan)
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			errMsg = fmt.Sprintf("Error occurred during node termination and deregistration: %v", err)
			cleanupAndFatal(clientset, config, errMsg)
		case duration := <-deregChan:
			instanceDeregTime = duration
		case duration := <-termChan:
			instanceTermTime = duration
		}
	}

	utilities.PrintSummary(instanceProvisioningTime, instanceRegistrationTime, podReadinessTime, instanceDeregTime, instanceTermTime)
}

// cleanupAndFatal attempts to delete the specified deployment from the given namespace
// and logs the provided error message before exiting the program. It is designed to ensure
// that a generated deployment is cleaned up in the event of a critical failure during execution.
func cleanupAndFatal(clientset *kubernetes.Clientset, config Config, errMsg string) {
	log.Printf(errMsg)
	if config.deploymentName == "" {
		if err := k8s.DeleteDeployment(clientset, config.containerName, config.namespace); err != nil {
			log.Printf("Failed to delete deployment during cleanup: %v", err)
		}
	}
	log.Fatalf("Exiting...")
}

func monitorForSigint(clientset *kubernetes.Clientset, config Config) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT)

	go func() {
			<-sigint
			errMsg := fmt.Sprintf("Received SIGINT, cleaning up...")
			cleanupAndFatal(clientset, config, errMsg)
	}()
}

// Command k8s-autoscaler-benchmarker orchestrates the setup, execution, and teardown
// of a Kubernetes deployment to benchmark either Cluster Autoscaler or Karpenter autoscaling functionalities.
// It determines the autoscaler type based on input flags, creates or uses an existing deployment, scales it,
// and then monitors the provisioning of nodes and readiness of pods.
// It concludes by scaling down the deployment and monitoring node deregistration and EC2 instance termination,
// before printing out a summary of the benchmark results to stdout.
func main() {
	config := parseFlags()

	clientset, ec2Svc := initializeClients(config.kubeconfigPath, config.awsProfile)

	monitorForSigint(clientset, config)

	labelSelector, tagKey, tagValue := determineAutoscalerType(config, clientset)

	executeBenchmark(clientset, ec2Svc, config, labelSelector, tagKey, tagValue)
}
