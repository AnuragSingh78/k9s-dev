// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// initLogExplorer replaces the former unselectable text dump with two
// independently navigable representations of the same bounded snapshot.
func (o *podOpsDock) initLogExplorer(bg tcell.Color) {
	o.logTable = tview.NewTable()
	o.logTable.SetFixed(1, 0)
	o.logTable.SetSelectable(true, false)
	o.logTable.SetBorder(true)
	o.logTable.SetBorderPadding(0, 0, 1, 1)
	o.logTable.SetBackgroundColor(bg)
	o.logTable.SetBorderColor(tcell.NewHexColor(0x23724a))
	o.logTable.SetBorderFocusColor(tcell.NewHexColor(0x5bf4aa))
	o.logTable.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.NewHexColor(0x07140d)).
		Background(tcell.NewHexColor(0x80e6bd)).
		Attributes(tcell.AttrBold))
	o.logTable.SetSelectedFunc(func(row, _ int) {
		if row <= 0 || row-1 >= len(o.logShown) {
			return
		}
		o.showLogDetail(o.logRows[o.logShown[row-1]])
	})

	o.logs.SetBorderFocusColor(tcell.NewHexColor(0x5bf4aa))
	o.logs.SetWrap(true)
	o.logs.SetScrollable(true)
	o.logs.SetBorderPadding(0, 0, 1, 1)
	o.logs.SetInputCapture(o.logKeyboard)
	o.logTable.SetInputCapture(o.logKeyboard)

	o.logSearch = tview.NewInputField()
	o.logSearch.SetLabel(" / Find: ")
	o.logSearch.SetFieldBackgroundColor(bg)
	o.logSearch.SetBackgroundColor(bg)
	o.logSearch.SetFieldTextColor(tcell.NewHexColor(0xa8eacb))
	o.logSearch.SetLabelColor(tcell.NewHexColor(0x5bf4aa))
	o.logSearch.SetChangedFunc(func(text string) {
		o.logFilter = text
		o.renderLogs()
	})
	o.logSearch.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			o.logSearch.SetText("")
		}
		// Enter keeps the visible filter, Escape clears it, and Tab continues
		// the same forward/backward pane cycle as the rest of Emerald.
		o.searching = false
		o.app.SetFocus(o.logFocus())
		switch key {
		case tcell.KeyTab:
			o.focusNext(1)
		case tcell.KeyBacktab:
			o.focusNext(-1)
		}
	})

	o.logPages = tview.NewPages()
	o.logPages.AddPage("compact", o.logTable, true, true)
	o.logPages.AddPage("pretty", o.logs, true, false)
	o.logPanel = tview.NewFlex().SetDirection(tview.FlexRow)
	o.logPanel.AddItem(o.logSearch, 1, 0, false)
	o.logPanel.AddItem(o.logPages, 0, 1, true)
	o.setLogStatus(" Select a pod to preview recent container logs.")
}

func (o *podOpsDock) logFocus() tview.Primitive {
	if o.logPretty {
		return o.logs
	}
	return o.logTable
}

func (o *podOpsDock) isLogFocus() bool {
	primitive := o.app.GetFocus()
	return primitive == o.logTable || primitive == o.logs
}

func (o *podOpsDock) focusables() []tview.Primitive {
	var pod tview.Primitive
	if o.source != nil {
		pod = o.source
	}
	return []tview.Primitive{o.nav, pod, o.details, o.logFocus(), o.events}
}

// highlightPane explicitly marks the current keyboard destination. The
// component border focus signal alone is not sufficiently visible in many
// terminals because composite K9s pages delegate their focus to inner tables.
func (o *podOpsDock) highlightPane(index int) {
	if o == nil || o.nav == nil || !o.visible {
		return
	}
	if index < 0 || index > 4 {
		index = 1
	}
	o.activePane = index
	normal := tcell.NewHexColor(0x23724a)
	active := tcell.NewHexColor(0x51ec89)
	// The concrete primitives expose embedded Box setters but their setter
	// methods return different concrete types, so assign them individually.
	o.nav.SetBorderColor(normal)
	o.details.SetBorderColor(normal)
	o.events.SetBorderColor(normal)
	o.logTable.SetBorderColor(normal)
	o.logs.SetBorderColor(normal)
	if o.source != nil {
		o.source.GetTable().SetBorderColor(normal)
	}
	o.nav.SetTitle(" NAVIGATION [Tab] ")
	o.details.SetTitle(" POD DOCTOR / SELECTED ")
	o.events.SetTitle(" POD EVENTS ")
	switch index {
	case 0:
		o.nav.SetBorderColor(active)
		o.nav.SetTitle(" ▶ NAVIGATION [ACTIVE] ")
	case 1:
		if o.source != nil {
			o.source.GetTable().SetBorderColor(active)
		}
	case 2:
		o.details.SetBorderColor(active)
		o.details.SetTitle(" ▶ POD DOCTOR [ACTIVE] ")
	case 3:
		o.logTable.SetBorderColor(active)
		o.logs.SetBorderColor(active)
	case 4:
		o.events.SetBorderColor(active)
		o.events.SetTitle(" ▶ POD EVENTS [ACTIVE] ")
	}
	o.refreshLogTitle()
}

// focusNext uses standard Tab/Shift+Tab, including when entering from a
// mouse-selected panel. No Ctrl combinations are needed.
func (o *podOpsDock) focusNext(direction int) {
	if !o.visible || o.source == nil {
		return
	}
	items := o.focusables()
	current := o.app.GetFocus()
	index := 1
	for i, item := range items {
		if current == item || (i == 1 && current == o.source.GetTable()) {
			index = i
			break
		}
	}
	for step := 0; step < len(items); step++ {
		index = (index + direction + len(items)) % len(items)
		if items[index] != nil {
			o.app.SetFocus(items[index])
			o.highlightPane(index)
			return
		}
	}
}

func (o *podOpsDock) setLogStatus(message string) {
	if o.logTable == nil {
		return
	}
	o.logRows = nil
	o.logShown = nil
	o.logTable.Clear()
	o.logTable.SetCell(0, 0, tview.NewTableCell("TIME").SetSelectable(false))
	o.logTable.SetCell(0, 1, tview.NewTableCell("LEVEL").SetSelectable(false))
	o.logTable.SetCell(0, 2, tview.NewTableCell("MESSAGE").SetSelectable(false))
	o.logTable.SetCell(1, 0, tview.NewTableCell(message))
	o.logs.SetText(message)
	o.refreshLogTitle()
}

func (o *podOpsDock) setLogEntries(rows []podLogEntry, container string, err error) {
	if err != nil {
		o.setLogStatus(" Logs unavailable: " + err.Error())
		return
	}
	o.logRows = rows
	o.logContainer = container
	o.renderLogs()
}

func (o *podOpsDock) visibleLogEntries() []podLogEntry {
	out := make([]podLogEntry, 0, len(o.logShown))
	for _, i := range o.logShown {
		out = append(out, o.logRows[i])
	}
	return out
}

func (o *podOpsDock) renderLogs() {
	if o.logTable == nil {
		return
	}
	o.logShown = o.logShown[:0]
	search := strings.ToLower(strings.TrimSpace(o.logFilter))
	for i, entry := range o.logRows {
		if o.logMode != "" && entry.level != o.logMode {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(entry.raw), search) {
			continue
		}
		o.logShown = append(o.logShown, i)
	}
	o.logTable.Clear()
	for col, title := range []string{"TIME", "LEVEL", "MESSAGE"} {
		cell := tview.NewTableCell(title).SetSelectable(false)
		cell.SetTextColor(tcell.NewHexColor(0x80e6bd))
		o.logTable.SetCell(0, col, cell)
	}
	for row, index := range o.logShown {
		e := o.logRows[index]
		// Click once on any cell to open the beautified entry. Defer the
		// transition until after tview finishes the mouse event and focus
		// changes; otherwise its default click handler steals focus back.
		clicked := func() bool {
			o.app.QueueUpdateDraw(func() { o.showLogDetail(e) })
			return true
		}
		o.logTable.SetCell(row+1, 0, tview.NewTableCell(e.timestamp).SetClickedFunc(clicked))
		level := tview.NewTableCell(e.level)
		level.SetTextColor(tcell.GetColor(previewLevelColor(e.level)))
		level.SetClickedFunc(clicked)
		o.logTable.SetCell(row+1, 1, level)
		msg := strings.ReplaceAll(e.message, "\n", " ")
		o.logTable.SetCell(row+1, 2, tview.NewTableCell(msg).SetClickedFunc(clicked))
	}
	if len(o.logShown) == 0 {
		o.logTable.SetCell(1, 0, tview.NewTableCell("No matching log entries (try 'a' or clear the filter)"))
	} else {
		o.logTable.Select(1, 0)
	}
	o.logs.SetText(renderPrettyLogPreview(o.visibleLogEntries()))
	o.logs.ScrollToBeginning()
	o.refreshLogTitle()
}

func (o *podOpsDock) refreshLogTitle() {
	if o.logTable == nil {
		return
	}
	mode := "COMPACT"
	if o.logPretty { mode = "PRETTY" }
	filter := "all"
	if o.logMode != "" { filter = strings.ToLower(o.logMode) }
	container := o.logContainer
	if len(container) > 20 { container = container[:20] + "…" }
	title := fmt.Sprintf(" LOGS %s | %d/%d | %d lines | %dKB | %s ",
		mode, len(o.logShown), len(o.logRows), o.logTail, o.logBytes/1024, filter)
	if container != "" { title += container + " " }
	o.logTable.SetTitle(title)
	o.logs.SetTitle(title)
}

func (o *podOpsDock) showLogDetail(entry podLogEntry) {
	detail := NewDetails(o.app, "Log Entry / Beautified", "Selected log", "", false)
	if err := o.app.inject(detail, false); err != nil {
		o.app.Flash().Err(err)
		return
	}
	detail.Update(renderSingleLogDetail(entry))
}

func (o *podOpsDock) refreshCurrentPod() {
	if o.source == nil { return }
	o.path = ""
	o.updateTable(o.source)
}

func (o *podOpsDock) resizeDock(delta int) {
	next := o.dockWeight + delta
	if next < 3 { next = 3 }
	if next > 16 { next = 16 }
	if next == o.dockWeight { return }
	o.dockWeight = next
	if o.app.cockpitRight != nil {
		o.app.cockpitRight.ResizeItem(o.dock, 0, next)
	}
}

func (o *podOpsDock) exportVisibleLogs() {
	if len(o.logShown) == 0 {
		o.app.Flash().Warn("No log records to export")
		return
	}
	dir := o.app.Config.K9s.ContextScreenDumpDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		o.app.Flash().Err(err)
		return
	}
	ns, name := client.Namespaced(o.path)
	filename := fmt.Sprintf("%s-%s-logs-%s.log", ns, name, time.Now().Format("20060102-150405"))
	file := filepath.Join(dir, filename)
	var b strings.Builder
	for _, row := range o.visibleLogEntries() {
		b.WriteString(row.raw)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(file, []byte(b.String()), 0600); err != nil {
		o.app.Flash().Err(err)
		return
	}
	o.app.Flash().Infof("Saved selected pod logs to %s (may contain application-sensitive data)", file)
}

// handleLogKey returns true when the event was handled. The app-level call
// comes before K9s global shortcuts (notably '-' and Enter), so log controls
// never activate an unrelated command while the log table is selected.
func (o *podOpsDock) handleLogKey(evt *tcell.EventKey) bool {
	if !o.visible || !o.isLogFocus() || o.app.InCmdMode() {
		return false
	}
	if evt.Key() == tcell.KeyCtrlS {
		o.exportVisibleLogs()
		return true
	}
	if evt.Key() == tcell.KeyEnter {
		if !o.logPretty {
			row, _ := o.logTable.GetSelection()
			if row > 0 && row-1 < len(o.logShown) {
				o.showLogDetail(o.logRows[o.logShown[row-1]])
			}
		} else {
			o.logPretty = false
			o.logPages.SwitchToPage("compact")
			o.app.SetFocus(o.logTable)
			o.refreshLogTitle()
		}
		return true
	}
	if evt.Key() != tcell.KeyRune {
		return false
	}
	switch evt.Rune() {
	case 'b':
		o.logPretty = !o.logPretty
		if o.logPretty {
			o.logPages.SwitchToPage("pretty")
		} else {
			o.logPages.SwitchToPage("compact")
		}
		o.app.SetFocus(o.logFocus())
		o.refreshLogTitle()
	case '/':
		o.searching = true
		o.app.SetFocus(o.logSearch)
	case '[', ']':
		direction := -1
		if evt.Rune() == ']' { direction = 1 }
		o.logTail = nextPreviewOption(previewTailOptions, o.logTail, direction)
		o.refreshLogTitle()
		o.refreshCurrentPod()
	case ',', '.':
		direction := -1
		if evt.Rune() == '.' { direction = 1 }
		o.logBytes = nextPreviewOption(previewByteOptions, o.logBytes, direction)
		o.refreshLogTitle()
		o.refreshCurrentPod()
	case '+', '=':
		o.resizeDock(1)
	case '-':
		o.resizeDock(-1)
	case 'r':
		o.refreshCurrentPod()
	case 'w':
		o.logMode = "WARN"
		o.renderLogs()
	case 'e':
		o.logMode = "ERROR"
		o.renderLogs()
	case 'i':
		o.logMode = "INFO"
		o.renderLogs()
	case 'd':
		o.logMode = "DEBUG"
		o.renderLogs()
	case 'a':
		o.logMode = ""
		o.renderLogs()
	case 'W':
		o.logWrap = !o.logWrap
		o.logs.SetWrap(o.logWrap)
	case 'c', 'C':
		row, _ := o.logTable.GetSelection()
		if row <= 0 || row-1 >= len(o.logShown) { return true }
		entry := o.logRows[o.logShown[row-1]]
		value := entry.raw
		if evt.Rune() == 'C' { value = prettyLogEntry(entry) }
		if err := clipboardWrite(value); err != nil {
			o.app.Flash().Err(err)
		} else {
			o.app.Flash().Info("Selected log entry copied")
		}
	case 'g':
		o.logTable.Select(1, 0)
		o.logs.ScrollToBeginning()
	case 'G':
		if len(o.logShown) > 0 {
			o.logTable.Select(len(o.logShown), 0)
		}
		o.logs.ScrollToEnd()
	case 'n':
		if len(o.logShown) > 0 {
			row, _ := o.logTable.GetSelection()
			o.logTable.Select(1+(row%len(o.logShown)), 0)
		}
	case 'N':
		if len(o.logShown) > 0 {
			row, _ := o.logTable.GetSelection()
			if row <= 1 { row = len(o.logShown) + 1 }
			o.logTable.Select(row-1, 0)
		}
	default:
		return false
	}
	return true
}

func (o *podOpsDock) logKeyboard(evt *tcell.EventKey) *tcell.EventKey {
	if o.handleLogKey(evt) {
		return nil
	}
	return evt
}

// logSummary helps regressions assert the active settings without depending on
// terminal layout or the user's selected skin.
func (o *podOpsDock) logSummary() string {
	return strconv.Itoa(o.logTail) + "/" + strconv.Itoa(o.logBytes)
}
