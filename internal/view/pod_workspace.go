// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/dao"
	"github.com/derailed/k9s/internal/model"
	"github.com/derailed/k9s/internal/ui"
	"github.com/derailed/k9s/internal/view/cmd"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	podWorkspaceName = "Pod Workspace"
	podFetchTimeout  = 12 * time.Second
	podLogLimit      = 128 * 1024
	podLogTail       = 80
)

// PodWorkspace provides an on-demand, split-panel diagnostic view.
// It intentionally does not open long-lived log streams or mutate resources.
type PodWorkspace struct {
	*tview.Flex
	app        *App
	path       string
	pod        *v1.Pod
	doctor     *tview.TextView
	events     *tview.TextView
	logs       *tview.TextView
	banner     *tview.TextView
	footer     *tview.TextView
	panels     []*tview.TextView
	active     int
	actions    *ui.KeyActions
	cancelLoad context.CancelFunc
}

// NewPodWorkspace creates the native K9s terminal split view.
func NewPodWorkspace(path string, doctorFirst bool) *PodWorkspace {
	p := &PodWorkspace{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		path:    path,
		actions: ui.NewKeyActions(),
	}
	p.banner = tview.NewTextView().SetDynamicColors(true)
	p.footer = tview.NewTextView().SetDynamicColors(true)
	p.doctor = tview.NewTextView().SetDynamicColors(true)
	p.events = tview.NewTextView().SetDynamicColors(true)
	p.logs = tview.NewTextView().SetDynamicColors(true)
	p.panels = []*tview.TextView{p.doctor, p.logs, p.events}
	if !doctorFirst {
		p.active = 1
	}
	return p
}

func (*PodWorkspace) SetCommand(*cmd.Interpreter)            {}
func (*PodWorkspace) SetFilter(string, bool)                 {}
func (*PodWorkspace) SetLabelSelector(labels.Selector, bool) {}
func (*PodWorkspace) InCmdMode() bool                        { return false }
func (p *PodWorkspace) Focus(delegate func(tview.Primitive)) { delegate(p.panels[p.active]) }
func (*PodWorkspace) ExtraHints() map[string]string          { return nil }
func (p *PodWorkspace) Name() string                         { return podWorkspaceName }
func (p *PodWorkspace) Hints() model.MenuHints               { return p.actions.Hints() }

func (p *PodWorkspace) Init(ctx context.Context) error {
	var err error
	if p.app, err = extractApp(ctx); err != nil {
		return err
	}
	ns, name := client.Namespaced(p.path)
	if ns == "" || name == "" {
		return fmt.Errorf("pod workspace requires a selected namespaced pod")
	}

	p.banner.SetText(fmt.Sprintf(" [#39ff14::b] EMERALD / POD WORKSPACE [-:-:-]  %s / %s", tview.Escape(ns), tview.Escape(name)))
	p.footer.SetText(" [#39ff14::b]Tab[-:-:-] panel   [#39ff14::b]R[-:-:-] refresh   [#39ff14::b]L[-:-:-] full logs   [#39ff14::b]E[-:-:-] all events   [#39ff14::b]Esc[-:-:-] back")
	p.doctor.SetTitle(" POD DOCTOR / DIAGNOSIS ")
	p.events.SetTitle(" POD EVENTS ")
	p.logs.SetTitle(" RECENT LOGS / SNAPSHOT ")

	left := tview.NewFlex().SetDirection(tview.FlexRow)
	left.AddItem(p.doctor, 0, 3, true)
	left.AddItem(p.events, 0, 2, false)
	pair := tview.NewFlex().SetDirection(tview.FlexColumn)
	pair.AddItem(left, 0, 2, true)
	pair.AddItem(p.logs, 0, 3, false)
	p.AddItem(p.banner, 1, 0, false)
	p.AddItem(pair, 0, 1, true)
	p.AddItem(p.footer, 1, 0, false)

	for _, panel := range p.panels {
		panel.SetBorder(true)
		panel.SetScrollable(true).SetWrap(true)
		panel.SetBorderPadding(0, 0, 1, 1)
		panel.SetInputCapture(p.keyboard)
	}
	p.bindKeys()
	p.StylesChanged(p.app.Styles)
	p.renderLoading()
	return nil
}

func (p *PodWorkspace) bindKeys() {
	p.actions.Bulk(ui.KeyMap{
		tcell.KeyEscape: ui.NewKeyAction("Back", p.app.PrevCmd, true),
		ui.KeyQ:         ui.NewKeyAction("Back", p.app.PrevCmd, false),
		tcell.KeyTab:    ui.NewKeyAction("Next Panel", p.nextPaneCmd, true),
		tcell.KeyBacktab: ui.NewKeyAction("Previous Panel", p.previousPaneCmd, true),
		ui.KeyR:         ui.NewKeyAction("Refresh Diagnostics", p.refreshCmd, true),
		ui.KeyL:         ui.NewKeyAction("Full Logs", p.fullLogsCmd, true),
		ui.KeyE:         ui.NewKeyAction("Namespace Events", p.fullEventsCmd, true),
	})
}

func (p *PodWorkspace) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	if a, ok := p.actions.Get(ui.AsKey(evt)); ok {
		return a.Action(evt)
	}
	return evt
}

func (p *PodWorkspace) nextPaneCmd(*tcell.EventKey) *tcell.EventKey {
	p.focusPane((p.active + 1) % len(p.panels))
	return nil
}

func (p *PodWorkspace) previousPaneCmd(*tcell.EventKey) *tcell.EventKey {
	p.focusPane((p.active + len(p.panels) - 1) % len(p.panels))
	return nil
}

func (p *PodWorkspace) focusPane(index int) {
	p.active = index
	p.app.SetFocus(p.panels[p.active])
}

// StylesChanged follows the user's active K9s skin (including custom skins).
func (p *PodWorkspace) StylesChanged(s *config.Styles) {
	p.SetBackgroundColor(s.BgColor())
	p.banner.SetBackgroundColor(s.BgColor())
	p.footer.SetBackgroundColor(s.BgColor())
	for _, panel := range p.panels {
		panel.SetBackgroundColor(s.BgColor())
		panel.SetTextColor(s.FgColor())
		panel.SetBorderColor(s.Frame().Border.FgColor.Color())
		panel.SetBorderFocusColor(s.Frame().Border.FocusColor.Color())
		panel.SetTitleColor(s.Frame().Title.FgColor.Color())
	}
}

func (p *PodWorkspace) renderLoading() {
	p.doctor.SetText(" [#39ff14::]Loading pod status and diagnostics...[-::]")
	p.events.SetText(" Loading recent events...")
	p.logs.SetText(" Loading the last 80 log lines...")
}

// Start loads a bounded, read-only snapshot off the terminal event loop.
func (p *PodWorkspace) Start() {
	p.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelLoad = cancel
	p.renderLoading()
	p.app.Styles.AddListener(p)
	go p.loadSnapshot(ctx)
}

func (p *PodWorkspace) Stop() {
	if p.cancelLoad != nil {
		p.cancelLoad()
		p.cancelLoad = nil
	}
	if p.app != nil {
		p.app.Styles.RemoveListener(p)
	}
}

func (p *PodWorkspace) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	p.Start()
	return nil
}

func (p *PodWorkspace) loadSnapshot(ctx context.Context) {
	reqCtx, cancel := context.WithTimeout(ctx, podFetchTimeout)
	defer cancel()
	ns, name := client.Namespaced(p.path)

	conn := p.app.Conn()
	if conn == nil {
		p.showError(ctx, "No Kubernetes connection configured")
		return
	}
	dial, err := conn.Dial()
	if err != nil {
		p.showError(ctx, "Unable to connect to Kubernetes: "+err.Error())
		return
	}
	pod, err := dial.CoreV1().Pods(ns).Get(reqCtx, name, metav1.GetOptions{})
	if err != nil {
		p.showError(ctx, "Unable to read pod: "+err.Error())
		return
	}

	var events []v1.Event
	var eventsErr error
	selector := fields.OneTermEqualSelector("involvedObject.name", name).String()
	result, err := dial.CoreV1().Events(ns).List(reqCtx, metav1.ListOptions{FieldSelector: selector, Limit: 100})
	if err != nil {
		eventsErr = err
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

	logText := fetchRecentPodLogs(reqCtx, dial, pod)
	report := renderPodDiagnosis(pod, events)
	eventText := renderPodEvents(events, eventsErr)

	if ctx.Err() != nil {
		return
	}
	p.app.QueueUpdateDraw(func() {
		if ctx.Err() != nil {
			return
		}
		p.pod = pod
		p.doctor.SetText(report)
		p.events.SetText(eventText)
		p.logs.SetText(logText)
		p.doctor.ScrollToBeginning()
		p.events.ScrollToBeginning()
		p.logs.ScrollToEnd()
	})
}

func (p *PodWorkspace) showError(ctx context.Context, msg string) {
	if ctx.Err() != nil {
		return
	}
	p.app.QueueUpdateDraw(func() {
		if ctx.Err() == nil {
			p.doctor.SetText("[#ff5b6e::b]POD DOCTOR UNAVAILABLE[-:-:-]\n"+tview.Escape(msg))
			p.events.SetText("Unavailable")
			p.logs.SetText("Unavailable")
		}
	})
}

func eventTimestamp(ev v1.Event) time.Time {
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	return ev.CreationTimestamp.Time
}

func renderPodDiagnosis(pod *v1.Pod, events []v1.Event) string {
	var b strings.Builder
	ready, restarts := 0, int32(0)
	for _, status := range pod.Status.ContainerStatuses {
		if status.Ready {
			ready++
		}
		restarts += status.RestartCount
	}
	fmt.Fprintf(&b, " [#39ff14::b]%s[-:-:-]\n", tview.Escape(pod.Name))
	fmt.Fprintf(&b, " Namespace : %s\n Node      : %s\n Phase     : %s\n Ready     : %d/%d\n Restarts  : %d\n",
		tview.Escape(pod.Namespace), tview.Escape(pod.Spec.NodeName),
		tview.Escape(string(pod.Status.Phase)), ready, len(pod.Spec.Containers), restarts)
	b.WriteString("\n [#39ff14::b]DIAGNOSTIC FINDINGS[-:-:-]\n")
	for _, f := range diagnosePod(pod, events) {
		tint := "#baffcc"
		switch f.level {
		case "ERROR":
			tint = "#ff5b6e"
		case "WARN", "EVENT":
			tint = "#ffca58"
		case "OK":
			tint = "#39ff14"
		}
		fmt.Fprintf(&b, "\n [%s::b]%s[-:-:-] %s\n", tint, f.level, tview.Escape(f.title))
		if f.detail != "" {
			fmt.Fprintf(&b, " %s\n", tview.Escape(f.detail))
		}
		fmt.Fprintf(&b, " [#79ffa2::]Next:[-::] %s\n", tview.Escape(f.next))
	}
	return b.String()
}

func renderPodEvents(events []v1.Event, listErr error) string {
	if listErr != nil {
		return "[#ffca58::b]Events unavailable[-:-:-]\n"+tview.Escape(listErr.Error())+"\n(Pod status and logs may still be available)"
	}
	if len(events) == 0 {
		return " No recent pod events found."
	}
	var b strings.Builder
	for i, ev := range events {
		if i == 20 {
			break
		}
		color := "#baffcc"
		if ev.Type == v1.EventTypeWarning {
			color = "#ffca58"
		}
		fmt.Fprintf(&b, " [%s::b]%s[-:-:-]  %s  %s\n", color, tview.Escape(ev.Type),
			tview.Escape(ev.Reason), eventTimestamp(ev).Local().Format("15:04:05"))
		fmt.Fprintf(&b, " %s\n\n", tview.Escape(strings.TrimSpace(ev.Message)))
	}
	return b.String()
}

// fetchRecentPodLogs is shared by the full-screen and inline pod diagnostic views.
func fetchRecentPodLogs(ctx context.Context, dial kubernetes.Interface, pod *v1.Pod) string {
	if len(pod.Spec.Containers) == 0 {
		return " No regular containers available for log preview."
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
	tail, limit := int64(podLogTail), int64(podLogLimit)
	opts := &v1.PodLogOptions{
		Container: container, TailLines: &tail,
		LimitBytes: &limit, Timestamps: true, Previous: previous,
	}
	data, err := dial.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts).Do(ctx).Raw()
	if err != nil {
		return fmt.Sprintf(" [#ffca58::b]Logs unavailable[-:-:-] (%s)\n %s",
			tview.Escape(container), tview.Escape(err.Error()))
	}
	var b strings.Builder
	if previous {
		b.WriteString(" [#ffca58::b]Showing previous container logs (CrashLoopBackOff)[-:-:-]\n")
	} else {
		b.WriteString(" [#39ff14::b]Snapshot: last 80 lines (not live stream)[-:-:-]\n")
	}
	fmt.Fprintf(&b, " Container: %s\n\n", tview.Escape(container))
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		shade := "#baffcc"
		switch {
		case strings.Contains(strings.ToUpper(line), "ERROR"), strings.Contains(strings.ToUpper(line), "FATAL"):
			shade = "#ff5b6e"
		case strings.Contains(strings.ToUpper(line), "WARN"):
			shade = "#ffca58"
		}
		fmt.Fprintf(&b, "[%s::]%s[-::]\n", shade, tview.Escape(line))
	}
	return b.String()
}

func (p *PodWorkspace) fullLogsCmd(*tcell.EventKey) *tcell.EventKey {
	if p.pod == nil {
		p.app.Flash().Warn("Pod snapshot is not loaded yet")
		return nil
	}
	opts := podLogOptions(p.app, p.path, false, &p.pod.ObjectMeta, &p.pod.Spec)
	if err := p.app.inject(NewLog(client.PodGVR, opts), false); err != nil {
		p.app.Flash().Err(err)
	}
	return nil
}

func (p *PodWorkspace) fullEventsCmd(*tcell.EventKey) *tcell.EventKey {
	ns, _ := client.Namespaced(p.path)
	p.app.gotoResource("events "+ns, "", false, true)
	return nil
}
