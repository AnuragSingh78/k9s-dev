// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"testing"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestEmeraldPulseSummary(t *testing.T) {
	gvrs := client.GVRs{client.PodGVR, client.DpGVR, client.CpuGVR, client.MemGVR}
	partial := map[string]model.HealthPoint{
		client.PodGVR.String(): {GVR: client.PodGVR, Total: 10, Faults: 3},
	}
	s := summarizePulse(partial, gvrs)
	assert.Equal(t, "SCANNING", s.label())
	assert.Equal(t, 7, s.healthy)
	assert.Equal(t, 3, s.faults)
	assert.Equal(t, 1, s.seen)
	assert.Equal(t, 2, s.total)
	assert.Equal(t, client.PodGVR, s.worst)

	partial[client.DpGVR.String()] = model.HealthPoint{GVR: client.DpGVR, Total: 2, Faults: 1}
	s = summarizePulse(partial, gvrs)
	assert.Equal(t, "ATTENTION", s.label())
	assert.Equal(t, 8, s.healthy)
	assert.Equal(t, 4, s.faults)
	assert.Equal(t, client.PodGVR, s.worst)

	partial[client.PodGVR.String()] = model.HealthPoint{GVR: client.PodGVR, Total: 10}
	partial[client.DpGVR.String()] = model.HealthPoint{GVR: client.DpGVR, Total: 2}
	s = summarizePulse(partial, gvrs)
	assert.Equal(t, "HEALTHY", s.label())
	assert.Equal(t, 12, s.healthy)
	assert.Zero(t, s.faults)
	assert.Nil(t, s.worst)
}

func TestEmeraldPulseSummaryClampsInvalidFaults(t *testing.T) {
	s := summarizePulse(map[string]model.HealthPoint{
		client.PodGVR.String(): {Total: 2, Faults: 5},
	}, client.GVRs{client.PodGVR})
	assert.Zero(t, s.healthy)
	assert.Equal(t, 2, s.faults)
}
