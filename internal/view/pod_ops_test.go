// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodOpsSummaryCountsVisibleRows(t *testing.T) {
	statuses := []string{"Running", "Running", "Pending", "CrashLoopBackOff", "Completed", "Running"}
	ready := []string{"1/1", "1/2", "0/1", "0/1", "0/1", "2/2"}
	restarts := []int{0, 1, 0, 3, 0, 0}
	got := podStatusCounts(statuses, ready, restarts)
	assert.Equal(t, 6, got.total)
	assert.Equal(t, 2, got.ready)
	assert.Equal(t, 3, got.issues)
	assert.Equal(t, 1, got.completed)
	assert.Equal(t, 4, got.restarts)
}

func TestPodOpsEmptyAndReadyValidation(t *testing.T) {
	assert.Equal(t, podCounts{}, podStatusCounts(nil, nil, nil))
	for _, tc := range []struct{ input string; want bool }{
		{"1/1", true}, {"2/2", true}, {" 1/2 ", false}, {"0/0", false},
		{"", false}, {"n/a", false}, {"1/x", false}, {"bogus/1", false},
	} {
		assert.Equal(t, tc.want, isPodReady(tc.input), tc.input)
	}
}

