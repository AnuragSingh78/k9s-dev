// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/model"
)

// pulseSnapshot summarizes only resource categories visible on the Pulse screen.
// Missing samples are not treated as healthy.
type pulseSnapshot struct {
	healthy int
	faults  int
	seen    int
	total   int
	worst   *client.GVR
}

func summarizePulse(points map[string]model.HealthPoint, gvrs client.GVRs) pulseSnapshot {
	s := pulseSnapshot{}
	for _, gvr := range gvrs {
		if gvr == client.CpuGVR || gvr == client.MemGVR {
			continue
		}
		s.total++
		p, ok := points[gvr.String()]
		if !ok {
			continue
		}
		s.seen++
		faults := p.Faults
		if faults < 0 {
			faults = 0
		}
		if faults > p.Total {
			faults = p.Total
		}
		s.healthy += p.Total - faults
		s.faults += faults
		if faults > 0 {
			if s.worst == nil || faults > points[s.worst.String()].Faults {
				s.worst = gvr
			}
		}
	}
	return s
}

func (s pulseSnapshot) label() string {
	if s.seen < s.total {
		return "SCANNING"
	}
	if s.faults > 0 {
		return "ATTENTION"
	}
	return "HEALTHY"
}
