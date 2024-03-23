// Copyright (c) 2024 Matthew Hopkins
// This file is part of the k8s-autoscaler-benchmarker project, under the MIT License.
// For full license text, see the LICENSE file in the root directory or https://github.com/moebaca/k8s-autoscaler-benchmarker.
package utilities

import (
	"bytes"
	"os"
	"testing"
	"time"
	"strings"
)

// TestPrintSummary checks that PrintSummary writes the expected strings to standard output.
func TestPrintSummary(t *testing.T) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintSummary(2*time.Second, 1*time.Second, 3*time.Second, 4*time.Second)

	w.Close()
	os.Stdout = old
	buf.ReadFrom(r)
	output := buf.String()

	expectedStrings := []string{
		"Benchmarks Summary",
		"EC2 Instance Initiation Time:",
		"2.00 seconds",
		"Pod Readiness Time:",
		"1.00 seconds",
		"Node Deregistration Time:",
		"3.00 seconds",
		"Node Termination Time:",
		"4.00 seconds",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("PrintSummary() did not write the expected string: got %s, wanted it to contain %s", output, expected)
		}
	}
}

// TestInt32Ptr checks that Int32Ptr returns a non-nil pointer to an int32 and that the value is correct.
func TestInt32Ptr(t *testing.T) {
	i := int32(42)
	ptr := Int32Ptr(i)
	if ptr == nil {
		t.Errorf("Int32Ptr() returned nil, want pointer to %d", i)
	}
	if *ptr != i {
		t.Errorf("Int32Ptr() returned a pointer to %d, want %d", *ptr, i)
	}
}
