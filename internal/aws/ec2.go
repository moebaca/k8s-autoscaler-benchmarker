// Package aws provides AWS service interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package aws

import (
	"fmt"
	"time"

	"github.com/moebaca/k8s-autoscaler-benchmarker/internal/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// GetEC2Instances retrieves a list of EC2 instances based on the specified filter.
func GetEC2Instances(ec2Svc *ec2.EC2, filterName, filterValue string) ([]*ec2.Instance, error) {
	var instances []*ec2.Instance
	backoffDuration := config.BaseBackoffDelay

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(filterName),
				Values: []*string{aws.String(filterValue)},
			},
		},
	}

	for retries := 0; retries < config.MaxRetries; retries++ {
		err := ec2Svc.DescribeInstancesPages(input, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			for _, reservation := range page.Reservations {
				for _, instance := range reservation.Instances {
					if instance.LaunchTime.After(config.ProgramStartTime) &&
						*instance.State.Name != ec2.InstanceStateNameTerminated {
						instances = append(instances, instance)
					}
				}
			}
			return !lastPage
		})

		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "Throttling" {
				fmt.Printf("Throttling error encountered, backing off for %v\n", backoffDuration)
				time.Sleep(backoffDuration)
				backoffDuration *= 2
				continue
			}
			return nil, fmt.Errorf("failed to describe EC2 instances: %w", err)
		}
		break
	}

	return instances, nil
}

// FormatInstanceDetails returns a slice of formatted instance details strings.
func FormatInstanceDetails(instances []*ec2.Instance) []string {
	details := make([]string, 0, len(instances))
	for _, instance := range instances {
		detail := fmt.Sprintf("%s (%s)", *instance.InstanceId, *instance.PrivateDnsName)
		details = append(details, detail)
	}
	return details
}

// GetInstanceState returns the current state of an EC2 instance.
func GetInstanceState(instance *ec2.Instance) string {
	if instance == nil || instance.State == nil || instance.State.Name == nil {
		return ""
	}
	return *instance.State.Name
}

// IsInstancePending checks if an instance is in the 'pending' state.
func IsInstancePending(instance *ec2.Instance) bool {
	return GetInstanceState(instance) == ec2.InstanceStateNamePending
}

// IsInstanceRunning checks if an instance is in the 'running' state.
func IsInstanceRunning(instance *ec2.Instance) bool {
	return GetInstanceState(instance) == ec2.InstanceStateNameRunning
}

// IsInstanceTerminated checks if an instance is in the 'terminated' state.
func IsInstanceTerminated(instance *ec2.Instance) bool {
	return GetInstanceState(instance) == ec2.InstanceStateNameTerminated
}
