// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/derailed/k9s/internal/dao"
	"github.com/derailed/tview"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	previewDefaultTail  = 80
	previewDefaultBytes = 128 * 1024
)

var (
	previewTailOptions = []int{20, 50, 80, 100, 200, 500}
	previewByteOptions = []int{64 * 1024, 128 * 1024, 256 * 1024, 512 * 1024, 1024 * 1024}
	previewLevels = regexp.MustCompile("(?i)\\b(ERROR|FATAL|PANIC|SEVERE|CRITICAL|WARN|WARNING|INFO|DEBUG|TRACE)\\b")
)

// podLogEntry retains the complete raw line, even when the list displays only
// its condensed message. This also preserves Kubernetes timestamps.
type podLogEntry struct {
	raw, timestamp, level, message, pretty string
	json bool
}

func parsePodLogEntry(raw string) podLogEntry {
	out := podLogEntry{raw: raw, message: raw, level: "TEXT"}
	line := strings.TrimSpace(raw)
	if i := strings.IndexByte(line, ' '); i > 0 {
		if stamp, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			out.timestamp = stamp.Local().Format("15:04:05.000")
			line = strings.TrimSpace(line[i+1:])
		}
	}
	if out.timestamp == "" {
		out.timestamp = "--:--:--"
	}
	out.message = line
	payload := line
	// Handle a single JSON object or array, including JSON after a Kubernetes
	// timestamp. Do not assume arbitrary text or stack traces are JSON.
	if len(payload) > 0 && (payload[0] == '{' || payload[0] == '[') && json.Valid([]byte(payload)) {
		var b bytes.Buffer
		if err := json.Indent(&b, []byte(payload), "", "  "); err == nil {
			out.json, out.pretty = true, b.String()
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(payload), &fields) == nil {
			for _, key := range []string{"level", "severity", "loglevel", "logLevel"} {
				if data, ok := fields[key]; ok {
					var v string
					if json.Unmarshal(data, &v) == nil && v != "" {
						out.level = normalizePreviewLevel(v)
						break
					}
				}
			}
			for _, key := range []string{"message", "msg", "error", "event"} {
				if data, ok := fields[key]; ok {
					var v string
					if json.Unmarshal(data, &v) == nil && v != "" {
						out.message = v
						break
					}
				}
			}
		}
	}
	if out.level == "TEXT" {
		if match := previewLevels.FindString(line); match != "" {
			out.level = normalizePreviewLevel(match)
		} else {
			out.level = "INFO"
		}
	}
	return out
}

func normalizePreviewLevel(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "ERR", "ERROR", "SEVERE", "FATAL", "CRITICAL", "PANIC":
		return "ERROR"
	case "WARN", "WARNING":
		return "WARN"
	case "DEBUG", "TRACE":
		return "DEBUG"
	default:
		return "INFO"
	}
}

func previewLevelColor(level string) string {
	switch level {
	case "ERROR":
		return "#ff5b6e"
	case "WARN":
		return "#ffca58"
	case "DEBUG":
		return "#659678"
	default:
		return "#6fffa0"
	}
}

func nextPreviewOption(options []int, current, direction int) int {
	index := 0
	for i, value := range options {
		if value >= current {
			index = i
			break
		}
		if i == len(options)-1 {
			index = i
		}
	}
	if direction < 0 && options[index] >= current && index > 0 {
		index--
	} else if direction > 0 && options[index] <= current && index < len(options)-1 {
		index++
	}
	return options[index]
}

// fetchPodLogEntries retrieves a bounded, read-only Kubernetes log snapshot.
// It does not open a watch, and never collects container environment variables.
func fetchPodLogEntries(ctx context.Context, dial kubernetes.Interface, pod *v1.Pod, tailCount, byteLimit int) ([]podLogEntry, string, error) {
	if len(pod.Spec.Containers) == 0 {
		return nil, "", fmt.Errorf("pod has no regular containers")
	}
	container := pod.Spec.Containers[0].Name
	if preferred, ok := dao.GetDefaultContainer(&pod.ObjectMeta, &pod.Spec); ok {
		container = preferred
	}
	previous := false
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container && status.RestartCount > 0 &&
			status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff" {
			previous = true
			break
		}
	}
	if tailCount < 1 { tailCount = previewDefaultTail }
	if byteLimit < 1 { byteLimit = previewDefaultBytes }
	tail, limit := int64(tailCount), int64(byteLimit)
	opts := &v1.PodLogOptions{
		Container: container, TailLines: &tail,
		LimitBytes: &limit, Timestamps: true, Previous: previous,
	}
	data, err := dial.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts).Do(ctx).Raw()
	if err != nil {
		return nil, container, err
	}
	if len(data) == 0 {
		return nil, container, nil
	}
	rows := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(rows) > tailCount {
		rows = rows[len(rows)-tailCount:]
	}
	out := make([]podLogEntry, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row) != "" {
			out = append(out, parsePodLogEntry(row))
		}
	}
	if previous { container += " [previous]" }
	return out, container, nil
}

func prettyLogEntry(entry podLogEntry) string {
	if !entry.json {
		return entry.raw
	}
	return entry.pretty
}

// renderPrettyLogPreview creates a readable whole-pane view; dynamic markup
// from arbitrary Kubernetes logs is escaped before reaching tview.
func renderPrettyLogPreview(entries []podLogEntry) string {
	if len(entries) == 0 {
		return " No recent log entries returned."
	}
	var b strings.Builder
	for i, entry := range entries {
		fmt.Fprintf(&b, " [#71977d::]%03d  %s[-::]  [%s::b]%s[-::]\n",
			i+1, tview.Escape(entry.timestamp), previewLevelColor(entry.level), tview.Escape(entry.level))
		if entry.json {
			for _, line := range strings.Split(entry.pretty, "\n") {
				fmt.Fprintf(&b, " [#8affb7::]%s[-::]\n", tview.Escape(line))
			}
		} else {
			fmt.Fprintf(&b, " %s\n", tview.Escape(entry.message))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderSingleLogDetail(entry podLogEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[#39ff14::b]LOG ENTRY / BEAUTIFIED[-:-:-]\n\n")
	fmt.Fprintf(&b, "[#71977d::]TIME[-::]  %s\n", tview.Escape(entry.timestamp))
	fmt.Fprintf(&b, "[#71977d::]LEVEL[-::] [%s::b]%s[-::]\n\n", previewLevelColor(entry.level), entry.level)
	b.WriteString("[#39ff14::b]MESSAGE / STRUCTURED DATA[-:-:-]\n")
	for _, line := range strings.Split(prettyLogEntry(entry), "\n") {
		fmt.Fprintf(&b, " %s\n", tview.Escape(line))
	}
	b.WriteString("\n[#39ff14::b]ORIGINAL RAW LINE[-:-:-]\n")
	fmt.Fprintf(&b, " %s\n", tview.Escape(entry.raw))
	b.WriteString("\n[#659678::]Esc/q: back  |  c: copy visible content[-::]")
	return b.String()
}
