// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package aws provides functions to interact with AWS services, specifically for
// managing and querying EC2 instances as part of the k8s-autoscaler-benchmarker application.
package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetEC2Instances retrieves a list of EC2 instances based on the specified filter name and value.
// It uses the DescribeInstancesPages function from the AWS SDK to paginate through
// all instances that match the filter criteria.
func GetEC2Instances(ec2Svc *ec2.EC2, filterName, filterValue string) ([]*ec2.Instance, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(filterName),
				Values: []*string{aws.String(filterValue)},
			},
		},
	}

	var instances []*ec2.Instance
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

	return instances, err
}

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
