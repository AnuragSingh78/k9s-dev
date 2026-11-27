// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorRejectsInvalidTimeout(t *testing.T) {
	err := runDoctor(context.Background(), io.Discard, doctorOptions{timeout: 0})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout validation error, got %v", err)
	}
}

func TestDoctorMissingKubeconfig(t *testing.T) {
	err := runDoctor(context.Background(), io.Discard, doctorOptions{
		kubeconfig: filepath.Join(t.TempDir(), "missing-kubeconfig"),
		timeout: 5 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "kubeconfig") {
		t.Fatalf("expected missing kubeconfig error, got %v", err)
	}
}

func TestDoctorUnknownContext(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	content := []byte(`apiVersion: v1
kind: Config
clusters:
  - name: local
    cluster:
      server: https://127.0.0.1:6443
contexts:
  - name: known
    context:
      cluster: local
      user: dev
current-context: known
users:
  - name: dev
    user: {}
`)
	if err := os.WriteFile(config, content, 0600); err != nil {
		t.Fatal(err)
	}

	err := runDoctor(context.Background(), io.Discard, doctorOptions{
		kubeconfig: config,
		kubeContext: "does-not-exist",
		timeout: 5 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected unknown context error, got %v", err)
	}
}
