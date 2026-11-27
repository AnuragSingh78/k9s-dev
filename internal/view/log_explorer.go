// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	logErrorsPattern   = `\b(ERROR|FATAL|PANIC|SEVERE|CRITICAL)\b`
	logWarningsPattern = `\b(WARN|WARNING)\b`
)

func explorerFilter(mode, manual string) string {
	switch mode {
	case "ERROR":
		return logErrorsPattern
	case "WARN":
		return logWarningsPattern
	default:
		return manual
	}
}

// prettyJSONLogLine formats only complete JSON lines; streaming text, ANSI
// output and Kubernetes-prefixed records pass through unchanged.
func prettyJSONLogLine(line []byte) []byte {
	raw := bytes.TrimSpace(line)
	if len(raw) == 0 || (raw[0] != '{' && raw[0] != '[') || !json.Valid(raw) {
		return line
	}
	var dst bytes.Buffer
	if err := json.Indent(&dst, raw, "", "  "); err != nil {
		return line
	}
	if bytes.HasSuffix(line, []byte("\n")) {
		dst.WriteByte('\n')
	}
	return dst.Bytes()
}

func explorerTitle(mode string) string {
	if mode == "" {
		return ""
	}
	return " " + strings.ToUpper(mode) + " ONLY"
}
