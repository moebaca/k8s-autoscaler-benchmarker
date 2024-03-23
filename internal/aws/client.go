// Package aws provides AWS service interaction for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// NewEC2Client creates a new EC2 client using the specified AWS profile.
func NewEC2Client(awsProfile string) (*ec2.EC2, error) {
	// Create session options with the specified profile
	opts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Profile:           awsProfile,
	}

	// Create session
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create EC2 client
	ec2Svc := ec2.New(sess)

	// Verify credentials by making a simple API call
	_, err = ec2Svc.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to validate AWS credentials for profile '%s': %w", awsProfile, err)
	}

	return ec2Svc, nil
}
