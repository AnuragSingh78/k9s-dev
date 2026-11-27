// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/model"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

// podCounts are counts of VISIBLE pod table rows, not cluster-wide inventory.
type podCounts struct {
	total, ready, issues, completed, restarts int
}

func isPodReady(value string) bool {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return false
	}
	ready, errReady := strconv.Atoi(parts[0])
	all, errAll := strconv.Atoi(parts[1])
	return errReady == nil && errAll == nil && all > 0 && ready == all
}

func podStatusCounts(statuses, readiness []string, restarts []int) podCounts {
	var c podCounts
	for i, status := range statuses {
		c.total++
		status = strings.ToLower(strings.TrimSpace(status))
		if status == "completed" || status == "succeeded" {
			c.completed++
		} else if status == "running" && i < len(readiness) && isPodReady(readiness[i]) {
			c.ready++
		} else {
			c.issues++
		}
		if i < len(restarts) && restarts[i] > 0 {
			c.restarts += restarts[i]
		}
	}
	return c
}

// podOpsDock is a real split-pane dashboard that lives on the main Pods
// screen, rather than a page only accessible via a special shortcut.
type podOpsDock struct {
	app       *App
	banner    *tview.TextView
	nav       *tview.List
	kpis      *tview.Flex
	cards     []*tview.TextView
	dock      *tview.Flex
	details   *tview.TextView
	logs      *tview.TextView
	logTable  *tview.Table
	logPages  *tview.Pages
	logPanel  *tview.Flex
	logSearch *tview.InputField
	logRows   []podLogEntry
	logShown  []int
	logTail   int
	logBytes  int
	logMode   string
	logFilter string
	logContainer string
	logPretty bool
	logWrap   bool
	searching bool
	dockWeight int
	activePane int
	statusText string
	statusLevel model.FlashLevel
	events    *tview.TextView
	source    *Pod
	path      string
	cancel    context.CancelFunc
	visible   bool
	collapsed bool
}

func newPodOpsDock(app *App) *podOpsDock {
	o := &podOpsDock{
		app: app, logTail: previewDefaultTail,
		logBytes: previewDefaultBytes, logWrap: true, dockWeight: 6,
		activePane: 1,
	}
	bg := app.Styles.BgColor()
	o.banner = tview.NewTextView().SetDynamicColors(true)
	o.banner.SetBackgroundColor(bg)

	o.nav = tview.NewList().ShowSecondaryText(false)
	o.nav.SetBorder(true).SetTitle(" NAVIGATION [Tab] ")
	o.nav.SetBackgroundColor(bg)
	o.nav.SetMainTextColor(app.Styles.FgColor())
	o.nav.SetSelectedBackgroundColor(tcell.NewHexColor(0x38d58d))
	o.nav.SetSelectedTextColor(tcell.NewHexColor(0x07140d))
	entries := []struct{ title, command string }{
		{"1  Pods", "pods"},
		{"2  Deployments", "deployments"},
		{"3  StatefulSets", "statefulsets"},
		{"4  DaemonSets", "daemonsets"},
		{"5  Services", "services"},
		{"6  Ingress", "ingresses"},
		{"7  ConfigMaps", "configmaps"},
		{"8  Events", "events"},
		{"9  Nodes", "nodes"},
		{"0  Namespaces", "namespaces"},
		{"   PVCs", "persistentvolumeclaims"},
		{"   Helm", "helm"},
		{"   Pulse Health", "pulse"},
		{"   XRay", "xray"},
	}
	for _, entry := range entries {
		o.nav.AddItem(entry.title, "", 0, nil)
	}
	o.nav.SetSelectedFunc(func(index int, _, _ string, _ rune) {
		if index < 0 || index >= len(entries) {
			return
		}
		o.app.gotoResource(entries[index].command, "", true, true)
	})
	o.nav.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		if evt.Key() == tcell.KeyEscape {
			if top := o.app.Content.Top(); top != nil {
				o.app.SetFocus(top)
			}
			return nil
		}
		return evt
	})

	o.kpis = tview.NewFlex().SetDirection(tview.FlexColumn)
	for _, label := range []string{"VISIBLE PODS", "READY", "ISSUES", "COMPLETED", "RESTARTS"} {
		card := tview.NewTextView().SetDynamicColors(true)
		card.SetBorder(true).SetTitle(" " + label + " ")
		card.SetBackgroundColor(bg)
		card.SetBorderColor(tcell.NewHexColor(0x23724a))
		card.SetTextAlign(tview.AlignCenter)
		card.SetText("[#5f936b::]—[-::]")
		o.cards = append(o.cards, card)
		o.kpis.AddItem(card, 0, 1, false)
	}
	o.details = tview.NewTextView().SetDynamicColors(true)
	o.logs = tview.NewTextView().SetDynamicColors(true)
	o.events = tview.NewTextView().SetDynamicColors(true)
	o.initLogExplorer(bg)
	o.details.SetTitle(" POD DOCTOR / SELECTED ")
	o.logs.SetTitle(" RECENT LOGS [snapshot] ")
	o.events.SetTitle(" POD EVENTS ")
	for _, panel := range []*tview.TextView{o.details, o.logs, o.events} {
		panel.SetBorder(true)
		panel.SetBackgroundColor(bg)
		panel.SetBorderColor(tcell.NewHexColor(0x23724a))
		panel.SetBorderPadding(0, 0, 1, 1)
		panel.SetScrollable(true).SetWrap(true)
		panel.SetTextColor(app.Styles.FgColor())
	}
	o.dock = tview.NewFlex().SetDirection(tview.FlexColumn)
	o.dock.AddItem(o.details, 0, 3, false)
	o.dock.AddItem(o.logPanel, 0, 4, false)
	o.dock.AddItem(o.events, 0, 3, false)
	o.reset()
	return o
}

func (o *podOpsDock) reset() {
	o.details.SetText(" No pod selected. If the Pods table remains empty, press Ctrl+R to refresh.\n Check the active context, namespace, filters and list permissions.\n Compare with: kubectl get pods -n <namespace>")
	o.setLogStatus(" Select a pod to preview recent container logs.")
	o.events.SetText(" Select a pod to inspect its warning and normal events.")
}

func (o *podOpsDock) stop() {
	if o.cancel != nil {
		o.cancel()
		o.cancel = nil
	}
	o.source = nil
	o.path = ""
}

func (o *podOpsDock) refreshBanner(namespace string) {
	contextName := "unknown"
	if o.app.Conn() != nil {
		contextName = o.app.Conn().ActiveContext()
	}
	banner := fmt.Sprintf(" [#39ff14::b]◆ K9S DEV / EMERALD OPS[-:-:-]   [#baffcc::]Context:[-::] %s   [#baffcc::]Namespace:[-::] %s   [#39ff14::]Tab[-::] move   [#39ff14::]Shift+B[-::] full table",
		tview.Escape(contextName), tview.Escape(namespace))
	if o.statusText != "" {
		tint := "#baffcc"
		switch o.statusLevel {
		case model.FlashWarn:
			tint = "#ffca58"
		case model.FlashErr:
			tint = "#ff5b6e"
		}
		// Keep the actionable portion readable on narrow terminal windows.
		msg := o.statusText
		if len(msg) > 100 {
			msg = msg[:100] + "..."
		}
		banner += fmt.Sprintf("  [%s::b]%s[-:-:-]", tint, tview.Escape(msg))
	}
	o.banner.SetText(banner)
}

// setStatus ensures pod-watch and Kubernetes errors remain visible. The
// standard K9s logo is deliberately hidden by the compact Emerald header.
func (o *podOpsDock) setStatus(level model.FlashLevel, msg string) {
	o.statusLevel, o.statusText = level, msg
	o.refreshBanner(o.app.Config.ActiveNamespace())
}

func (o *podOpsDock) clearStatus() {
	o.statusText = ""
	o.refreshBanner(o.app.Config.ActiveNamespace())
}

// refreshView is called on the terminal event loop when Pods becomes active.
func (o *podOpsDock) refreshView(p *Pod) {
	o.source = p
	if p == nil {
		return
	}
	o.refreshBanner(o.app.Config.ActiveNamespace())
	o.updateTable(p)
	o.highlightPane(o.activePane)
}

func (o *podOpsDock) updateTable(p *Pod) {
	if !o.visible || p == nil || o.source != p {
		return
	}
	tbl := p.GetTable()
	statusCol, hasStatus := tbl.HeaderIndex("STATUS")
	readyCol, hasReady := tbl.HeaderIndex("READY")
	restartCol, hasRestarts := tbl.HeaderIndex("RESTARTS")
	if !hasStatus {
		return
	}
	var statuses, ready []string
	var restarts []int
	for row := 1; row < tbl.GetRowCount(); row++ {
		statusCell := tbl.GetCell(row, statusCol)
		if statusCell == nil {
			continue
		}
		statuses = append(statuses, statusCell.Text)
		readiness := ""
		if hasReady {
			if c := tbl.GetCell(row, readyCol); c != nil {
				readiness = c.Text
			}
		}
		ready = append(ready, readiness)
		restartCount := 0
		if hasRestarts {
			if c := tbl.GetCell(row, restartCol); c != nil {
				restartCount, _ = strconv.Atoi(strings.TrimSpace(c.Text))
			}
		}
		restarts = append(restarts, restartCount)
	}
	o.showCounts(podStatusCounts(statuses, ready, restarts))
	path := tbl.GetSelectedItem()
	if path == o.path {
		return
	}
	o.path = path
	o.stopRequest()
	if path == "" {
		o.reset()
		return
	}
	o.details.SetText(" [#39ff14::]Inspecting selected pod...[-::]")
	o.setLogStatus(" Loading bounded log snapshot...")
	o.events.SetText(" Loading pod events...")
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel
	go o.loadSnapshot(ctx, path, o.logTail, o.logBytes)
}

func (o *podOpsDock) showCounts(c podCounts) {
	vals := []struct{ n int; color string }{
		{c.total, "#d9ffe2"},
		{c.ready, "#39ff14"},
		{c.issues, "#ffca58"},
		{c.completed, "#71977d"},
		{c.restarts, "#ffca58"},
	}
	for i, v := range vals {
		o.cards[i].SetText(fmt.Sprintf("[%s::b]%d[-::]", v.color, v.n))
	}
}

func (o *podOpsDock) stopRequest() {
	if o.cancel != nil {
		o.cancel()
		o.cancel = nil
	}
}

// loadSnapshot uses the same Kubernetes read requests as the standalone Pod
// Workspace. A short selection debounce prevents flooding the API on arrow-key
// navigation; an older selection can never overwrite the newly selected pod.
func (o *podOpsDock) loadSnapshot(ctx context.Context, path string, tailCount, byteLimit int) {
	timer := time.NewTimer(175 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	ctxReq, cancel := context.WithTimeout(ctx, podFetchTimeout)
	defer cancel()
	ns, name := client.Namespaced(path)
	if o.app.Conn() == nil {
		o.showError(ctx, "No Kubernetes connection available")
		return
	}
	dial, err := o.app.Conn().Dial()
	if err != nil {
		o.showError(ctx, err.Error())
		return
	}
	pod, err := dial.CoreV1().Pods(ns).Get(ctxReq, name, metav1.GetOptions{})
	if err != nil {
		o.showError(ctx, err.Error())
		return
	}
	events := make([]v1.Event, 0, 12)
	var eventErr error
	selector := fields.OneTermEqualSelector("involvedObject.name", name).String()
	result, err := dial.CoreV1().Events(ns).List(ctxReq, metav1.ListOptions{FieldSelector: selector, Limit: 100})
	if err != nil {
		eventErr = err
	} else {
		for _, ev := range result.Items {
			if ev.InvolvedObject.Kind == "Pod" && (ev.InvolvedObject.UID == "" || ev.InvolvedObject.UID == pod.UID) {
				events = append(events, ev)
			}
		}
	}
	sort.Slice(events, func(i, j int) bool {
		return eventTimestamp(events[i]).After(eventTimestamp(events[j]))
	})
	diagnosis := renderPodDiagnosis(pod, events)
	if eventErr != nil {
		diagnosis += "\n [#ffca58::]Event access unavailable; some findings may be missing.[-::]"
	}
	eventText := renderPodEvents(events, eventErr)
	logRows, logContainer, logErr := fetchPodLogEntries(ctxReq, dial, pod, tailCount, byteLimit)
	if ctx.Err() != nil {
		return
	}
	o.app.QueueUpdateDraw(func() {
		if ctx.Err() != nil || !o.visible || o.path != path {
			return
		}
		o.details.SetText(diagnosis)
		o.setLogEntries(logRows, logContainer, logErr)
		o.events.SetText(eventText)
		o.details.ScrollToBeginning()
		o.logs.ScrollToBeginning()
		o.events.ScrollToBeginning()
	})
}

func (o *podOpsDock) showError(ctx context.Context, detail string) {
	if ctx.Err() != nil {
		return
	}
	o.app.QueueUpdateDraw(func() {
		if ctx.Err() != nil || !o.visible {
			return
		}
		o.details.SetText("[#ff5b6e::b]Unable to inspect pod[-:-:-]\n"+tview.Escape(detail))
		o.setLogStatus("Pod log snapshot unavailable")
		o.events.SetText("Pod events unavailable")
	})
}
