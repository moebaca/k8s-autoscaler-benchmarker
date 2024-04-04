// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package aws provides functions to interact with AWS services, specifically for
// managing and querying EC2 instances as part of the k8s-autoscaler-benchmarker application.
package aws

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"k8s.io/client-go/kubernetes"
)

// GetEC2Instances retrieves a list of EC2 instances based on the specified filter name and value,
// with an exponential backoff mechanism in case of throttling.
func GetEC2Instances(ec2Svc *ec2.EC2, filterName, filterValue string) ([]*ec2.Instance, error) {
	var instances []*ec2.Instance
	var backoffDuration = 1 * time.Second
	const maxRetries = 5

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(filterName),
				Values: []*string{aws.String(filterValue)},
			},
		},
	}

	for retries := 0; retries < maxRetries; retries++ {
		err := ec2Svc.DescribeInstancesPages(input, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			for _, reservation := range page.Reservations {
				for _, instance := range reservation.Instances {
					if *instance.State.Name != ec2.InstanceStateNameTerminated {
						instances = append(instances, instance)
					}
				}
			}
			return !lastPage
		})

		if err != nil {
			awsErr, ok := err.(awserr.Error)
			if ok && awsErr.Code() == "Throttling" {
				fmt.Printf("Throttling error encountered, backing off for %v\n", backoffDuration)
				time.Sleep(backoffDuration)
				backoffDuration *= 2
			} else {
				return nil, err
			}
		} else {
			break
		}
	}

	return instances, nil
}

// MonitorInstanceProvisioning tracks the provisioning status of EC2 instances by filtering with tag key and value.
// It prompts the user for action if provisioning exceeds the predefined timeout.
// The function logs the provisioning status, including launched instances, until all required instances are in a 'Pending' state or a user intervention occurs.
func MonitorInstanceProvisioning(clientset kubernetes.Interface, ec2Svc *ec2.EC2, tagKey, tagValue, deploymentName, namespace string) (time.Duration, int, error) {
	fmt.Println("Monitoring EC2 instance provisioning...")
	var instanceDetails []string
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
									return time.Since(startTime), instanceCount, fmt.Errorf("Failed to read input: %w", err)
							}
							answer = strings.TrimSpace(answer)
							if answer == "no" {
									return time.Since(startTime), instanceCount, fmt.Errorf("Exiting due to user input.")
							} else if answer == "yes" {
									startTime = time.Now()
									fmt.Println("Please input 'no' at next timeout instead of force closing so that cleanup steps can be run by the program...")
									break
							} else {
									fmt.Println("Invalid input. Please enter 'yes' or 'no'.")
							}
					}
			}

			instances, err := GetEC2Instances(ec2Svc, "tag:"+tagKey, tagValue)
			if err != nil {
					return time.Since(startTime), instanceCount, fmt.Errorf("Error retrieving EC2 instances: %w", err)
			}

			if len(instances) > 0 && *instances[0].State.Name == ec2.InstanceStateNamePending {
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
