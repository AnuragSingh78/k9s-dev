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
			`[#80e6bd::b]api[#e5fff1::-]: [#bcdeca::]fred
		   [#80e6bd::b]version[#e5fff1::-]: [#bcdeca::]v1`,
		},
		{
			`api: <<<"search_0">>>fred<<<"">>>
		   version: v1`,
			`[#80e6bd::b]api[#e5fff1::-]: [#bcdeca::]["search_0"]fred[""]
		   [#80e6bd::b]version[#e5fff1::-]: [#bcdeca::]v1`,
		},
		{
			`api:
			version: v1`,
			`[#80e6bd::b]api[#e5fff1::-]:
			[#80e6bd::b]version[#e5fff1::-]: [#bcdeca::]v1`,
		},
		{
			"      fred:blee",
			"[#bcdeca::]      fred:blee",
		},
		{
			"fred blee: blee",
			"[#80e6bd::b]fred blee[#e5fff1::-]: [#bcdeca::]blee",
		},
		{
			"Node-Selectors:  <none>",
			"[#80e6bd::b]Node-Selectors[#e5fff1::-]: [#bcdeca::] <none>",
		},
		{
			"fred.blee:  <none>",
			"[#80e6bd::b]fred.blee[#e5fff1::-]: [#bcdeca::] <none>",
		},
		{
			"certmanager.k8s.io/cluster-issuer: nameOfClusterIssuer",
			"[#80e6bd::b]certmanager.k8s.io/cluster-issuer[#e5fff1::-]: [#bcdeca::]nameOfClusterIssuer",
		},
		{
			"Message: Pod The node was low on resource: [DiskPressure].",
			"[#80e6bd::b]Message[#e5fff1::-]: [#bcdeca::]Pod The node was low on resource: [DiskPressure[].",
		},
		{
			`data: "<<<"`,
			`[#80e6bd::b]data[#e5fff1::-]: [#bcdeca::]"<<<"`,
		},
	}

	s := config.NewStyles()
	for _, u := range uu {
		assert.Equal(t, u.e, colorizeYAML(s.Views().Yaml, u.s))
	}
}
