// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogExplorerQuickFilters(t *testing.T) {
	assert.Equal(t, "request-id=abc", explorerFilter("", "request-id=abc"))
	assert.Equal(t, logErrorsPattern, explorerFilter("ERROR", "request-id=abc"))
	assert.Equal(t, logWarningsPattern, explorerFilter("WARN", "request-id=abc"))

	rx, err := regexp.Compile("(?i)" + explorerFilter("ERROR", ""))
	assert.NoError(t, err)
	assert.True(t, rx.MatchString("2026-11-25 ERROR request failed"))
	assert.True(t, rx.MatchString("fatal: unable to connect"))
	assert.False(t, rx.MatchString("0 errors processed"))
	assert.False(t, rx.MatchString("WARNING retrying"))
}

func TestPrettyJSONLogLine(t *testing.T) {
	assert.Equal(t, "{\n  \"level\": \"error\",\n  \"message\": \"failed\"\n}\n",
		string(prettyJSONLogLine([]byte(`{"level":"error","message":"failed"}`+"\n"))))
	assert.Equal(t, "some plain text\n", string(prettyJSONLogLine([]byte("some plain text\n"))))
	assert.Equal(t, "2026-11-25 {\"a\":1}\n", string(prettyJSONLogLine([]byte("2026-11-25 {\"a\":1}\n"))))
	assert.Equal(t, "{invalid", string(prettyJSONLogLine([]byte("{invalid"))))
	assert.Equal(t, "", explorerTitle(""))
	assert.Equal(t, " ERROR ONLY", explorerTitle("ERROR"))
}
