// Package bench provides benchmark orchestration for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package bench

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/client-go/kubernetes"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/aws"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/config"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/k8s"
	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/util"
)

// BenchmarkResult holds the timing results of a benchmark run.
type BenchmarkResult struct {
	ProvisioningTime   time.Duration
	RegistrationTime   time.Duration
	PodReadinessTime   time.Duration
	DeregistrationTime time.Duration
	TerminationTime    time.Duration
}

// RunBenchmark orchestrates the complete benchmark process.
func RunBenchmark(ctx context.Context, cfg *config.Config) error {
	// Initialize clients
	clientset, ec2Svc, err := initializeClients(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize clients: %w", err)
	}

	// Determine autoscaler type and get appropriate selectors
	autoscalerType, labelSelector, tagKey, tagValue, err := setupAutoscaler(ctx, clientset, cfg)
	if err != nil {
		return fmt.Errorf("failed to setup autoscaler: %w", err)
	}
	fmt.Printf("Testing with %s autoscaler...\n", autoscalerType)

	// Initialize deployment config
	deploymentCfg := &k8s.DeploymentConfig{
		DeploymentName:    cfg.DeploymentName,
		Namespace:         cfg.Namespace,
		ContainerName:     cfg.ContainerName,
		ContainerImage:    cfg.ContainerImage,
		CPURequest:        cfg.CPURequest,
		TolerationsKey:    cfg.TolerationsKey,
		TolerationsValue:  cfg.TolerationsValue,
		NodeSelectorKey:   cfg.NodeSelectorKey,
		NodeSelectorValue: cfg.NodeSelectorValue,
		Replicas:          cfg.Replicas,
	}

	// Run the benchmark
	result, err := executeBenchmark(ctx, clientset, ec2Svc, deploymentCfg, labelSelector, tagKey, tagValue)
	if err != nil {
		return fmt.Errorf("benchmark execution failed: %w", err)
	}

	// Print results
	util.PrintSummary(
		result.ProvisioningTime,
		result.RegistrationTime,
		result.PodReadinessTime,
		result.DeregistrationTime,
		result.TerminationTime,
	)

	return nil
}

// initializeClients creates the necessary Kubernetes and AWS clients.
func initializeClients(cfg *config.Config) (*kubernetes.Clientset, *ec2.EC2, error) {
	// Create Kubernetes client
	clientset, err := k8s.NewClient(cfg.KubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("kubernetes client initialization failed: %w", err)
	}

	// Create AWS EC2 client
	ec2Svc, err := aws.NewEC2Client(cfg.AWSProfile)
	if err != nil {
		return nil, nil, fmt.Errorf("AWS client initialization failed: %w", err)
	}

	return clientset, ec2Svc, nil
}

// setupAutoscaler determines which autoscaler to use and returns appropriate selectors.
func setupAutoscaler(ctx context.Context, clientset *kubernetes.Clientset, cfg *config.Config) (string, string, string, string, error) {
	// Get autoscaler type
	autoscalerType, err := k8s.GetAutoscalerType(cfg.NodepoolTag, cfg.NodeGroup)
	if err != nil {
		return "", "", "", "", err
	}

	// Get label selector for Kubernetes operations
	labelSelector, err := k8s.GetLabelSelectorForAutoscaler(cfg.NodepoolTag, cfg.NodeGroup)
	if err != nil {
		return "", "", "", "", err
	}

	// Get tag key for EC2 operations
	tagKey, err := k8s.GetTagKeyForAutoscaler(cfg.NodepoolTag, cfg.NodeGroup)
	if err != nil {
		return "", "", "", "", err
	}

	// Get tag value
	tagValue, err := k8s.GetTagValueForAutoscaler(cfg.NodepoolTag, cfg.NodeGroup)
	if err != nil {
		return "", "", "", "", err
	}

	// Verify autoscaler is ready
	if err := k8s.VerifyAutoscalerReady(ctx, clientset, cfg.NodepoolTag, cfg.NodeGroup); err != nil {
		return "", "", "", "", err
	}

	fmt.Printf("Using node label selector: %s\n", labelSelector)
	return autoscalerType, labelSelector, tagKey, tagValue, nil
}

// executeBenchmark runs the actual benchmark and returns timing results.
func executeBenchmark(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	ec2Svc *ec2.EC2,
	deploymentCfg *k8s.DeploymentConfig,
	labelSelector, tagKey, tagValue string,
) (*BenchmarkResult, error) {
	result := &BenchmarkResult{}

	// Setup deployment - create new or scale existing
	if err := setupAndCleanupDeployment(ctx, clientset, deploymentCfg); err != nil {
		return nil, err
	}

	// Phase 1: Monitor instance provisioning
	// Fixed the call to match the expected signature
	provTime, instanceCount, err := aws.MonitorInstanceProvisioning(
		clientset,
		ec2Svc,
		tagKey,
		tagValue,
		deploymentCfg.DeploymentName,
		deploymentCfg.Namespace,
	)
	if err != nil {
		return nil, fmt.Errorf("instance provisioning failed: %w", err)
	}
	result.ProvisioningTime = provTime
	fmt.Printf("Instances provisioned in %s\n", util.FormatDuration(provTime))

	// Phase 2: Monitor node registration
	regTime, err := k8s.MonitorNodeRegistration(ctx, clientset, labelSelector, instanceCount)
	if err != nil {
		return nil, fmt.Errorf("node registration monitoring failed: %w", err)
	}
	result.RegistrationTime = regTime
	fmt.Printf("Nodes registered in %s\n", util.FormatDuration(regTime))

	// Phase 3: Wait for pods to become ready
	podTime, err := k8s.WaitForPodsReady(ctx, clientset, deploymentCfg.DeploymentName, deploymentCfg.Namespace, deploymentCfg.Replicas)
	if err != nil {
		return nil, fmt.Errorf("pod readiness monitoring failed: %w", err)
	}
	result.PodReadinessTime = podTime
	fmt.Printf("Pods became ready in %s\n", util.FormatDuration(podTime))

	// Phase 4: Scale down deployment to trigger node deregistration
	if err := k8s.ScaleDeployment(ctx, clientset, deploymentCfg.DeploymentName, deploymentCfg.Namespace, 0); err != nil {
		return nil, fmt.Errorf("failed to scale down deployment: %w", err)
	}

	// Phase 5: Monitor node deregistration and instance termination in parallel
	deregTime, termTime, err := monitorDeregistrationAndTermination(ctx, clientset, ec2Svc, deploymentCfg.NodeSelectorKey, deploymentCfg.NodeSelectorValue, tagKey, tagValue)
	if err != nil {
		return nil, err
	}

	result.DeregistrationTime = deregTime
	result.TerminationTime = termTime

	return result, nil
}

// setupAndCleanupDeployment creates/scales the deployment and ensures cleanup.
func setupAndCleanupDeployment(ctx context.Context, clientset *kubernetes.Clientset, cfg *k8s.DeploymentConfig) error {
	// Create or scale deployment
	if err := k8s.SetupDeployment(ctx, clientset, cfg); err != nil {
		return err
	}

	// If we created a new deployment (vs. using existing), set up cleanup
	if cfg.DeploymentName == "" || cfg.DeploymentName == cfg.ContainerName {
		// Set actual deployment name if it was empty
		if cfg.DeploymentName == "" {
			cfg.DeploymentName = cfg.ContainerName
		}

		// Ensure cleanup on exit
		go func() {
			<-ctx.Done()
			cleanupCtx := context.Background()
			if err := k8s.DeleteDeployment(cleanupCtx, clientset, cfg.DeploymentName, cfg.Namespace); err != nil {
				log.Printf("Warning: failed to delete deployment during cleanup: %v", err)
			}
		}()
	}

	return nil
}

// monitorDeregistrationAndTermination monitors node deregistration and termination in parallel.
func monitorDeregistrationAndTermination(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	ec2Svc *ec2.EC2,
	nodeSelectorKey, nodeSelectorValue string,
	tagKey, tagValue string,
) (time.Duration, time.Duration, error) {
	var deregTime, termTime time.Duration
	var wg sync.WaitGroup

	// Create channels for results and errors
	deregChan := make(chan time.Duration, 1)
	termChan := make(chan time.Duration, 1)
	errChan := make(chan error, 2)

	// Monitor node deregistration
	wg.Add(1)
	go func() {
		defer wg.Done()
		duration, err := k8s.MonitorNodeDeregistration(ctx, clientset, nodeSelectorKey, nodeSelectorValue)
		if err != nil {
			errChan <- fmt.Errorf("node deregistration monitoring failed: %w", err)
			return
		}
		deregChan <- duration
	}()

	// Monitor instance termination
	wg.Add(1)
	go func() {
		defer wg.Done()
		duration, err := aws.MonitorInstanceTermination(ctx, ec2Svc, tagKey, tagValue)
		if err != nil {
			errChan <- fmt.Errorf("instance termination monitoring failed: %w", err)
			return
		}
		termChan <- duration
	}()

	// Wait for both goroutines to complete
	go func() {
		wg.Wait()
		close(errChan)
		close(deregChan)
		close(termChan)
	}()

	// Collect results
	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				return 0, 0, err
			}
		case duration := <-deregChan:
			deregTime = duration
			fmt.Printf("Nodes deregistered in %s\n", util.FormatDuration(duration))
		case duration := <-termChan:
			termTime = duration
			fmt.Printf("Instances terminated in %s\n", util.FormatDuration(duration))
		}
	}

	return deregTime, termTime, nil
}
