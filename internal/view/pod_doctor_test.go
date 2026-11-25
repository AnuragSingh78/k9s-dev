// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"strings"
	"testing"

	"github.com/derailed/k9s/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestPodDoctorHealthy(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-gateway", Namespace: "default", UID: types.UID("pod-1")},
		Spec: v1.PodSpec{Containers: []v1.Container{{Name: "gateway"}}},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{{Name: "gateway", Ready: true}},
		},
	}
	findings := diagnosePod(pod, nil)
	require.Len(t, findings, 1)
	assert.Equal(t, "OK", findings[0].level)
	assert.Contains(t, renderPodDiagnosis(pod, nil), "No obvious issues")
}

func TestPodDoctorSchedulingImagePullAndEvents(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "broken", Namespace: "default", UID: types.UID("pod-2")},
		Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			Conditions: []v1.PodCondition{{
				Type: v1.PodScheduled, Status: v1.ConditionFalse,
				Message: "0/3 nodes available: insufficient memory",
			}},
			ContainerStatuses: []v1.ContainerStatus{{
				Name: "app", State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}},
		},
	}
	events := []v1.Event{
		{Type: v1.EventTypeWarning, Reason: "Failed", Message: "image pull failed",
			InvolvedObject: v1.ObjectReference{Kind: "Pod", UID: types.UID("pod-2")}},
		{Type: v1.EventTypeWarning, Reason: "OldPod", Message: "not related",
			InvolvedObject: v1.ObjectReference{Kind: "Pod", UID: types.UID("pod-1")}},
	}
	findings := diagnosePod(pod, events)
	var titles []string
	for _, f := range findings { titles = append(titles, f.title) }
	all := strings.Join(titles, " ")
	assert.Contains(t, all, "Pod not scheduled")
	assert.Contains(t, all, "ImagePullBackOff")
	assert.Contains(t, all, "Event: Failed")
	assert.NotContains(t, all, "OldPod")
}

func TestPodDoctorCrashloopAndOOM(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", UID: types.UID("pod-3")},
		Spec: v1.PodSpec{Containers: []v1.Container{{Name: "worker"}}},
		Status: v1.PodStatus{Phase: v1.PodRunning, ContainerStatuses: []v1.ContainerStatus{{
			Name: "worker", RestartCount: 5,
			State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			LastTerminationState: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{
				Reason: "OOMKilled", ExitCode: 137,
			}},
		}}},
	}
	findings := diagnosePod(pod, nil)
	var report string
	for _, f := range findings { report += f.title + " " + f.next }
	assert.Contains(t, report, "CrashLoopBackOff")
	assert.Contains(t, report, "OOMKilled")
	assert.Contains(t, report, "memory usage")
	assert.Contains(t, report, "previous container logs")
}

func TestPodWorkspaceIsValidAndKeyboardDriven(t *testing.T) {
	p := NewPodWorkspace("default/api-gateway", true)
	require.NoError(t, p.Init(makeContext(t)))
	assert.Equal(t, podWorkspaceName, p.Name())
	assert.False(t, p.InCmdMode())
	assert.Equal(t, 3, len(p.panels))
	assert.Equal(t, 0, p.active)
	_, ok := p.actions.Get(ui.KeyR)
	assert.True(t, ok)
	_, ok = p.actions.Get(ui.KeyL)
	assert.True(t, ok)
	p.nextPaneCmd(nil)
	assert.Equal(t, 1, p.active)
	p.Stop()
}

func TestPodWorkspaceRejectsMissingNamespace(t *testing.T) {
	p := NewPodWorkspace("not-namespaced", true)
	assert.Error(t, p.Init(makeContext(t)))
}
