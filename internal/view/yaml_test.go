// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"testing"

	"github.com/derailed/k9s/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestYaml(t *testing.T) {
	uu := []struct {
		s, e string
	}{
		{
			`api: fred
		   version: v1`,
			`[#79ffa2::b]api[#d9ffe2::-]: [#b0eabb::]fred
		   [#79ffa2::b]version[#d9ffe2::-]: [#b0eabb::]v1`,
		},
		{
			`api: <<<"search_0">>>fred<<<"">>>
		   version: v1`,
			`[#79ffa2::b]api[#d9ffe2::-]: [#b0eabb::]["search_0"]fred[""]
		   [#79ffa2::b]version[#d9ffe2::-]: [#b0eabb::]v1`,
		},
		{
			`api:
			version: v1`,
			`[#79ffa2::b]api[#d9ffe2::-]:
			[#79ffa2::b]version[#d9ffe2::-]: [#b0eabb::]v1`,
		},
		{
			"      fred:blee",
			"[#b0eabb::]      fred:blee",
		},
		{
			"fred blee: blee",
			"[#79ffa2::b]fred blee[#d9ffe2::-]: [#b0eabb::]blee",
		},
		{
			"Node-Selectors:  <none>",
			"[#79ffa2::b]Node-Selectors[#d9ffe2::-]: [#b0eabb::] <none>",
		},
		{
			"fred.blee:  <none>",
			"[#79ffa2::b]fred.blee[#d9ffe2::-]: [#b0eabb::] <none>",
		},
		{
			"certmanager.k8s.io/cluster-issuer: nameOfClusterIssuer",
			"[#79ffa2::b]certmanager.k8s.io/cluster-issuer[#d9ffe2::-]: [#b0eabb::]nameOfClusterIssuer",
		},
		{
			"Message: Pod The node was low on resource: [DiskPressure].",
			"[#79ffa2::b]Message[#d9ffe2::-]: [#b0eabb::]Pod The node was low on resource: [DiskPressure[].",
		},
		{
			`data: "<<<"`,
			`[#79ffa2::b]data[#d9ffe2::-]: [#b0eabb::]"<<<"`,
		},
	}

	s := config.NewStyles()
	for _, u := range uu {
		assert.Equal(t, u.e, colorizeYAML(s.Views().Yaml, u.s))
	}
}
