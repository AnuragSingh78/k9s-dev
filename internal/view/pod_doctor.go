// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// podFinding is a read-only diagnostic derived from observable pod status and
// events. Remediation is guidance only; Pod Doctor never mutates workloads.
type podFinding struct {
	level, title, detail, next string
}

func diagnosePod(pod *v1.Pod, events []v1.Event) []podFinding {
	if pod == nil {
		return []podFinding{{"UNKNOWN", "Pod not available", "Could not read pod state", "Check Kubernetes connectivity and RBAC permissions"}}
	}
	var findings []podFinding
	add := func(level, title, detail, next string) {
		findings = append(findings, podFinding{level, title, detail, next})
	}
	if pod.DeletionTimestamp != nil {
		add("WARN", "Termination in progress", "Pod has a deletion timestamp", "Check finalizers, grace period and events")
	}
	if pod.Status.Phase == v1.PodFailed {
		add("ERROR", "Pod failed", pod.Status.Message, "Check container exit codes and events")
	}
	if pod.Status.Phase == v1.PodPending && pod.Spec.NodeName == "" {
		scheduled := ""
		for _, c := range pod.Status.Conditions {
			if c.Type == v1.PodScheduled && c.Status == v1.ConditionFalse {
				scheduled = strings.TrimSpace(c.Message)
				break
			}
		}
		if scheduled != "" {
			add("WARN", "Pod not scheduled", scheduled, "Review node capacity, taints, affinity and PVC binding")
		} else {
			add("WARN", "Waiting for scheduling", "Pod has no assigned node", "Check scheduler events and node readiness")
		}
	}
	checkContainers := func(items []v1.ContainerStatus) {
		for _, cs := range items {
			prefix := "Container " + cs.Name
			if waiting := cs.State.Waiting; waiting != nil && waiting.Reason != "" {
				next := "Inspect pod events and container logs"
				switch waiting.Reason {
				case "CrashLoopBackOff":
					next = "Open previous container logs; check startup configuration and dependencies"
				case "ImagePullBackOff", "ErrImagePull":
					next = "Verify image name/tag, registry connectivity and image-pull secret"
				case "CreateContainerConfigError":
					next = "Check referenced Secrets, ConfigMaps and container configuration"
				case "ContainerCreating":
					next = "Inspect volume mounts, image pulling and scheduling events"
				}
				if waiting.Reason != "ContainerCreating" {
					add("WARN", prefix+": "+waiting.Reason, waiting.Message, next)
				}
			}
			if terminated := cs.State.Terminated; terminated != nil && terminated.ExitCode != 0 {
				add("ERROR", prefix+" exited", fmt.Sprintf("Exit code %d (%s)", terminated.ExitCode, terminated.Reason),
					"Inspect container logs and termination message")
			}
			if last := cs.LastTerminationState.Terminated; last != nil {
				if last.Reason == "OOMKilled" || last.ExitCode == 137 {
					add("ERROR", prefix+" OOMKilled", fmt.Sprintf("Previous exit %d, restarts %d", last.ExitCode, cs.RestartCount),
						"Inspect memory usage and limits; check for memory growth")
				} else if last.ExitCode != 0 && cs.RestartCount > 0 {
					add("WARN", prefix+" restarted", fmt.Sprintf("Previous exit %d, restarts %d", last.ExitCode, cs.RestartCount),
						"Inspect previous container logs and readiness/startup probes")
				}
			} else if cs.RestartCount >= 3 {
				add("WARN", prefix+" restarts", fmt.Sprintf("%d restarts reported", cs.RestartCount),
					"Inspect container logs and health probes")
			}
		}
	}
	checkContainers(pod.Status.InitContainerStatuses)
	checkContainers(pod.Status.ContainerStatuses)
	checkContainers(pod.Status.EphemeralContainerStatuses)

	for _, c := range pod.Status.Conditions {
		if c.Type == v1.PodReady && c.Status == v1.ConditionFalse && pod.Status.Phase == v1.PodRunning {
			add("WARN", "Pod not ready", strings.TrimSpace(c.Message),
				"Inspect container readiness, readiness probes and recent events")
		}
	}

	warnings := 0
	for _, ev := range events {
		if ev.Type != v1.EventTypeWarning || (ev.InvolvedObject.UID != "" && ev.InvolvedObject.UID != pod.UID) {
			continue
		}
		warnings++
		if warnings > 4 {
			break
		}
		add("EVENT", "Event: "+ev.Reason, ev.Message, "Correlate event time with pod/container state")
	}
	if len(findings) == 0 {
		add("OK", "No obvious issues", "No unhealthy status or warning events in this snapshot",
			"Review logs and metrics if the application still behaves unexpectedly")
	}
	return findings
}
