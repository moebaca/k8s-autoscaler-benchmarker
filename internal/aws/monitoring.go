// Package aws provides AWS service interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package aws

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/client-go/kubernetes"
)

// MonitorInstanceProvisioning monitors EC2 instance provisioning status.
// This signature matches the one expected in the error message.
func MonitorInstanceProvisioning(clientset kubernetes.Interface, ec2Svc *ec2.EC2, tagKey, tagValue, deploymentName, namespace string) (time.Duration, int, error) {
	fmt.Println("Monitoring EC2 instance provisioning...")
	startTime := time.Now()
	reader := bufio.NewReader(os.Stdin)
	timeout := 60 * time.Second
	instanceCount := 0

	for {
		time.Sleep(1 * time.Second)
		if time.Since(startTime) >= timeout {
			for {
				fmt.Println("Provisioning timeout exceeded. There may be an issue (check pod for errors). Do you want to continue waiting to troubleshoot issue? [yes/no]: ")
				answer, err := reader.ReadString('\n')
				if err != nil {
					return time.Since(startTime), instanceCount, fmt.Errorf("failed to read input: %w", err)
				}
				answer = strings.TrimSpace(answer)
				if answer == "no" {
					return time.Since(startTime), instanceCount, fmt.Errorf("exiting due to user input")
				} else if answer == "yes" {
					startTime = time.Now() // Reset the timer
					fmt.Println("Please input 'no' at next timeout instead of force closing so that cleanup steps can be run by the program...")
					break
				} else {
					fmt.Println("Invalid input. Please enter 'yes' or 'no'.")
				}
			}
		}

		instances, err := GetEC2Instances(ec2Svc, "tag:"+tagKey, tagValue)
		if err != nil {
			return time.Since(startTime), instanceCount, fmt.Errorf("error retrieving EC2 instances: %w", err)
		}

		if len(instances) > 0 && *instances[0].State.Name == ec2.InstanceStateNamePending {
			var instanceDetails []string
			for _, instance := range instances {
				detail := fmt.Sprintf("%s (%s)", *instance.InstanceId, *instance.PrivateDnsName)
				instanceDetails = append(instanceDetails, detail)
			}
			fmt.Println("Instances launched:", strings.Join(instanceDetails, ", "))
			instanceCount = len(instances) // Update instance count
			return time.Since(startTime), instanceCount, nil
		}
	}
}

// MonitorInstanceTermination watches EC2 instances until they're all terminated.
func MonitorInstanceTermination(ctx context.Context, ec2Svc *ec2.EC2, tagKey, tagValue string) (time.Duration, error) {
	fmt.Println("Monitoring EC2 instance termination...")
	startTime := time.Now()

	statusTicker := time.NewTicker(15 * time.Second)
	defer statusTicker.Stop()

	checkTicker := time.NewTicker(1 * time.Second)
	defer checkTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()

		case <-statusTicker.C:
			// Log current status periodically
			instances, err := GetEC2Instances(ec2Svc, "tag:"+tagKey, tagValue)
			if err != nil {
				return 0, fmt.Errorf("failed to list instances: %w", err)
			}

			if len(instances) > 0 {
				var instanceDetails []string
				for _, instance := range instances {
					instanceDetails = append(instanceDetails, fmt.Sprintf("%s (%s)", *instance.InstanceId, *instance.PrivateDnsName))
				}
				fmt.Println("EC2 instances still running:", strings.Join(instanceDetails, ", "))
			}

		case <-checkTicker.C:
			// Check if all instances are terminated
			instances, err := GetEC2Instances(ec2Svc, "tag:"+tagKey, tagValue)
			if err != nil {
				return 0, fmt.Errorf("failed to list instances: %w", err)
			}

			if len(instances) == 0 {
				fmt.Println("All EC2 instances have been terminated.")
				return time.Since(startTime), nil
			}
		}
	}
}
