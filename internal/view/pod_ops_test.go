// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/derailed/k9s/internal/config/mock"
	"github.com/derailed/tcell/v2"
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


func TestPodOpsTabFocusHighlights(t *testing.T) {
	app := NewApp(mock.NewMockConfig(t))
	dock := newPodOpsDock(app)
	dock.visible = true
	active := tcell.NewHexColor(0x51ec89)
	normal := tcell.NewHexColor(0x23724a)

	dock.highlightPane(0)
	assert.Equal(t, active, dock.nav.GetBorderColor())
	assert.Equal(t, normal, dock.details.GetBorderColor())
	assert.Contains(t, dock.nav.GetTitle(), "[ACTIVE]")

	dock.highlightPane(2)
	assert.Equal(t, active, dock.details.GetBorderColor())
	assert.Equal(t, normal, dock.nav.GetBorderColor())
	assert.Contains(t, dock.details.GetTitle(), "[ACTIVE]")

	dock.highlightPane(3)
	assert.Equal(t, active, dock.logTable.GetBorderColor())
	assert.Equal(t, active, dock.logs.GetBorderColor())
	assert.Contains(t, dock.logTable.GetTitle(), "[ACTIVE]")

	dock.highlightPane(4)
	assert.Equal(t, active, dock.events.GetBorderColor())
	assert.Equal(t, normal, dock.logTable.GetBorderColor())
	assert.Contains(t, dock.events.GetTitle(), "[ACTIVE]")
}
