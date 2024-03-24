// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.

// Package aws provides functions to interact with AWS services, specifically for
// managing and querying EC2 instances as part of the k8s-autoscaler-benchmarker application.
package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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
