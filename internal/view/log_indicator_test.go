// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view_test

import (
	"strings"
	"testing"

	"github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/view"
	"github.com/stretchr/testify/assert"
)

func TestLogIndicatorRefresh(t *testing.T) {
	defaults := config.NewStyles()
	uu := map[string]struct {
		li *view.LogIndicator
		e  string
	}{
		"all-containers": {
			view.NewLogIndicator(config.NewConfig(nil), defaults, true), "[::b]AllContainers:[#5f936b::d]Off[-::]     [::b]Autoscroll:[#39ff14::b]On[-::]      [::b]ColumnLock:[#5f936b::d]Off[-::]     [::b]FullScreen:[#5f936b::d]Off[-::]     [::b]Timestamps:[#5f936b::d]Off[-::]     [::b]Wrap:[#5f936b::d]Off[-::]\n",
		},
		"plain": {
			view.NewLogIndicator(config.NewConfig(nil), defaults, false), "[::b]Autoscroll:[#39ff14::b]On[-::]      [::b]ColumnLock:[#5f936b::d]Off[-::]     [::b]FullScreen:[#5f936b::d]Off[-::]     [::b]Timestamps:[#5f936b::d]Off[-::]     [::b]Wrap:[#5f936b::d]Off[-::]\n",
		},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			u.li.Refresh()
			expected := strings.TrimSuffix(u.e, "\n") + "\n [::b]Explorer[-::] ALL      [::b]JSON[-::] Off      [::d]E errors | W warnings | N reset | J pretty JSON[-::]\n"
			assert.Equal(t, expected, u.li.GetText(false))
		})
	}
}

func BenchmarkLogIndicatorRefresh(b *testing.B) {
	defaults := config.NewStyles()
	v := view.NewLogIndicator(config.NewConfig(nil), defaults, true)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		v.Refresh()
	}
}

func TestLogIndicatorExplorerModes(t *testing.T) {
	l := view.NewLogIndicator(config.NewConfig(nil), config.NewStyles(), false)
	l.SetExplorerMode("ERROR")
	l.SetPrettyJSON(true)
	assert.Contains(t, l.GetText(false), "Explorer[-::] ERROR")
	assert.Contains(t, l.GetText(false), "JSON[-::] On")
	l.SetExplorerMode("")
	l.SetPrettyJSON(false)
	assert.Contains(t, l.GetText(false), "Explorer[-::] ALL")
	assert.Contains(t, l.GetText(false), "JSON[-::] Off")
}
