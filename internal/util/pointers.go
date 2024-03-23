// Package util provides utility functions for the k8s-autoscaler-benchmarker.
// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
package util

// Int32Ptr returns a pointer to the given int32 value.
func Int32Ptr(i int32) *int32 {
	return &i
}

// StringPtr returns a pointer to the given string value.
func StringPtr(s string) *string {
	return &s
}

// BoolPtr returns a pointer to the given boolean value.
func BoolPtr(b bool) *bool {
	return &b
}

// Int64Ptr returns a pointer to the given int64 value.
func Int64Ptr(i int64) *int64 {
	return &i
}
