# Emerald Edition · Phase 4 — Interactive Log Explorer

> Implementation specification for the actual Go / tview / tcell terminal UI.

## Scope

The main Pods screen keeps the original K9s resource table, sidebar, health counters and Pod Doctor. It replaces the raw Recent Logs text dump with a selectable table and an alternate prettified view. The standalone Pod Workspace, full K9s log streaming viewer, shell, plugins, filters, context selection, RBAC and Shift+B full-table toggle remain supported.

The behavior is **read-only** unless the operator explicitly chooses to copy or export logs. No extra Kubernetes roles, background log watches or third-party services are added.

## Focus and navigation

Tab cycles **Sidebar → Pods → Pod Doctor → Recent Logs → Pod Events → Sidebar**. Shift+Tab goes backward. The active pane gets keyboard focus; arrows move or scroll within that pane. Sidebar Enter opens a native K9s resource view. Ctrl+N is no longer required. On other K9s screens, the upstream shortcuts remain unchanged.

## Phase 4 keybindings

| Context | Keys | Behavior |
|---|---|---|
| Cockpit | Tab / Shift+Tab | Switch pane forward / backward |
| Pods | Shift+B | Toggle original full-height table |
| Recent Logs (compact) | Up/Down, Enter | Select entry, then open full-screen beautified detail |
| Recent Logs (pretty) | Enter | Return to compact table |
| Recent Logs | b | Toggle beautified whole-pane mode |
| Recent Logs | / | Focus filter input, type to search, Enter to apply, Esc to clear |
| Recent Logs | a / e / w / i / d | All / errors / warnings / info / debug |
| Recent Logs | [ / ] | Select 20, 50, 80, 100, 200 or 500 tail lines |
| Recent Logs | , / . | Select 64, 128, 256, 512 or 1024 KiB maximum response |
| Recent Logs | + / - | Resize lower diagnostic area, bounded to 3–16 layout units |
| Recent Logs | r | Refresh selected-pod snapshot |
| Recent Logs | W | Toggle text wrapping in pretty mode |
| Recent Logs | g / G | Top / bottom of list or formatted text |
| Recent Logs | n / N | Next / previous visible entry in compact mode |
| Recent Logs (compact) | c / C | Copy raw / prettified selected log entry |
| Recent Logs | Ctrl+S | Export filtered raw lines into K9s screen-dump directory |
| Single log detail | Esc / q | Close detail, return to Pods cockpit |

Keyboard search is case-insensitive substring matching and does not remove records from the underlying snapshot. The filter is always visible above Recent Logs. Filtering by severity and search can be combined.

## Parsing and display

Each fetched log record keeps its original bytes as a string. Kubernetes RFC3339/RFC3339Nano timestamps are displayed in local time (HH:mm:ss.SSS); the raw timestamp stays intact. Severity is inferred from JSON level/severity fields or common words in unstructured text. Complete JSON objects or arrays after a timestamp are indented, and message/msg/error/event fields drive the compact summary. Incomplete JSON, non-JSON text and individual stacktrace lines remain intact. All user-controlled log text is escaped before it reaches tview's dynamic color parser.

The table shows TIME, LEVEL and MESSAGE with level-specific colors; Enter opens the existing native K9s Details screen showing the formatted record and its full raw original line. The alternate pretty pane shows each record indented and colored, with plain-text records left as text. This is not a web/CSS widget; it works in Windows Terminal.

## Data limits and life cycle

Default preview: **80 lines / 128 KiB**. Session-selectable limits: 20–500 lines and 64 KiB–1 MiB. Changing size triggers a new on-demand Kubernetes pod-log GET; it does not create a continuous watch. Switching the selected pod cancels previous reads and uses a short debounce to avoid request floods. Full streaming logs remain available using K9s's original log commands. The preview is per default/first container and uses previous logs for a restarting CrashLoopBackOff container.

## Security, regression and delivery

No automatic export or remote upload. Ctrl+S explicitly writes raw filtered logs to the configured K9s screen-dump directory with restrictive file permissions; the contents may include secrets or personal information written by applications, so operators should redact before sharing.

Tests cover JSON and plain text, severity, malformed input, raw retention, byte/line bounds, filtering, and safe markup escaping. CI must run the cmd, config and view test packages successfully, then cross-compile a standalone Windows x64 EXE and publish the artifact.

## Follow-on work, not claimed as completed

Persistent settings per Kubernetes context; multiline exception folding; merged multi-pod logs; in-pane streaming; and one-mouse-click detail activation (Enter works reliably now, including in terminals whose mouse driver does not emit the necessary events).
