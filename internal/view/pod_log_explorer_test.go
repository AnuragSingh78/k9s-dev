// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"strings"
	"testing"

	"github.com/derailed/k9s/internal/config/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewLogParsesPrefixedJSON(t *testing.T) {
	line := `2026-11-25T07:39:06.082538637Z {"level":"error","message":"connection refused","traceId":"abc-123","count":2}`
	entry := parsePodLogEntry(line)
	assert.True(t, entry.json)
	assert.Equal(t, "ERROR", entry.level)
	assert.Equal(t, "connection refused", entry.message)
	assert.Regexp(t, "^[0-9]{2}:[0-9]{2}:[0-9]{2}\\.[0-9]{3}$", entry.timestamp)
	assert.Contains(t, entry.pretty, "\n  \"traceId\":")
	assert.Equal(t, line, entry.raw)
}

func TestPreviewLogParsesPlainAndMalformedJSON(t *testing.T) {
	plain := parsePodLogEntry("2026-11-25T07:39:06Z WARN retrying")
	assert.Equal(t, "WARN", plain.level)
	assert.Equal(t, "WARN retrying", plain.message)
	assert.False(t, plain.json)

	broken := parsePodLogEntry("{invalid json")
	assert.False(t, broken.json)
	assert.Equal(t, "{invalid json", broken.raw)

	stack := parsePodLogEntry("at com.example.Worker.run(Worker.java:42)")
	assert.False(t, stack.json)
	assert.Equal(t, "INFO", stack.level)
}

func TestPreviewLogSettingsStayBounded(t *testing.T) {
	assert.Equal(t, 50, nextPreviewOption(previewTailOptions, 80, -1))
	assert.Equal(t, 100, nextPreviewOption(previewTailOptions, 80, 1))
	assert.Equal(t, 20, nextPreviewOption(previewTailOptions, 20, -1))
	assert.Equal(t, 500, nextPreviewOption(previewTailOptions, 500, 1))
	assert.Equal(t, 64*1024, nextPreviewOption(previewByteOptions, 128*1024, -1))
	assert.Equal(t, 256*1024, nextPreviewOption(previewByteOptions, 128*1024, 1))
	assert.Equal(t, 1024*1024, nextPreviewOption(previewByteOptions, 1024*1024, 1))
}

func TestPreviewLogBeautifiedDetailKeepsRawLine(t *testing.T) {
	entry := parsePodLogEntry(`{"level":"WARN","message":"db down","code":503}`)
	out := renderSingleLogDetail(entry)
	assert.Contains(t, out, "LOG ENTRY / BEAUTIFIED")
	assert.Contains(t, out, "ORIGINAL RAW LINE")
	assert.Contains(t, out, "db down")
	assert.Contains(t, out, "\"code\"")
	assert.Contains(t, out, "{\"level\":\"WARN\"")
	assert.Equal(t, "WARN", entry.level)
}

func TestPreviewLogRenderingEscapesUntrustedMarkers(t *testing.T) {
	entry := parsePodLogEntry("ERROR [red::b]attack[-::]")
	out := renderPrettyLogPreview([]podLogEntry{entry})
	assert.NotContains(t, out, " ERROR [red::b]attack")
	assert.Contains(t, out, "attack")
}

func TestPreviewLogTableFiltersWithoutDataLoss(t *testing.T) {
	app := NewApp(mock.NewMockConfig(t))
	o := newPodOpsDock(app)
	require.NotNil(t, o.logTable)
	o.setLogEntries([]podLogEntry{
		parsePodLogEntry("2026-11-25T07:40:00Z ERROR connection refused"),
		parsePodLogEntry("2026-11-25T07:40:01Z INFO started"),
		parsePodLogEntry("2026-11-25T07:40:02Z WARN failed probe"),
	}, "gateway", nil)
	assert.Len(t, o.logShown, 3)
	o.logMode = "ERROR"
	o.renderLogs()
	require.Len(t, o.logShown, 1)
	assert.Equal(t, 0, o.logShown[0])
	o.logMode = ""
	o.logFilter = "probe"
	o.renderLogs()
	require.Len(t, o.logShown, 1)
	assert.Equal(t, 2, o.logShown[0])
	assert.Len(t, o.logRows, 3)
	assert.Equal(t, "gateway", o.logContainer)
	assert.Contains(t, o.logTable.GetTitle(), "80 lines")
	o.logFilter = ""
	o.renderLogs()
	assert.Len(t, o.logShown, 3)
	assert.True(t, strings.Contains(o.logSummary(), "131072"))
}
