# K9s Dev — Emerald Edition

This build keeps the existing [K9s](https://k9scli.io/) terminal application and its Kubernetes features. It adds an **embedded emerald/forest-green UI**, keyboard navigation, a **Health Cockpit**, an **advanced log explorer**, resource aliases, a **read-only cluster doctor**, and an automated **Windows x64 `.exe` build**.

## Download and launch on Windows 11

1. Open [GitHub Actions](https://github.com/AnuragSingh78/k9s-dev/actions/workflows/build-windows-exe.yml).
2. Open the latest successful run on the feature branch (or on `main` after merge).
3. Under **Artifacts**, download `k9s-dev-windows-amd64` and extract `k9s-dev.exe`.
4. In **Windows Terminal** or **PowerShell**, from the download folder, run:

   ```powershell
   .\k9s-dev.exe version
   .\k9s-dev.exe doctor
   .\k9s-dev.exe
   ```

K9s is a **terminal user interface (TUI)**, not a traditional click-to-open Windows desktop app. Your Windows environment needs access to an existing Kubernetes kubeconfig (typically `%USERPROFILE%\.kube\config`) and the cluster API. If you only have a kubeconfig in WSL, either run the Linux build inside WSL or set up an accessible Windows kubeconfig. For Kubernetes hosted in WSL/minikube, API connectivity from Windows may require additional network configuration.

## UI design

The stock theme is now a bundled dark green theme:

- Deep forest background and readable mint text
- Emerald navigation borders, focused rows, prompts and breadcrumbs
- Teal/green resource charts, selected table rows and YAML highlights
- Amber warnings and red errors remain visually distinct

**This theme is compiled into the exe**, so first-time users do not have to copy skin files. Existing installations with `k9s.ui.skin` set to a non-stock custom theme continue using that theme; clear the custom skin setting or select `stock` to see Emerald.

## New navigation

Press `:` inside K9s and enter a command alias:

| Command | Opens |
|---|---|
| `:po`, `:dp`, `:sts`, `:ds` | Pods, deployments, StatefulSets, DaemonSets |
| `:svc`, `:ing`, `:ep` | Services, ingresses, endpoints |
| `:ns`, `:no`, `:ev` | Namespaces, nodes, events |
| `:cm`, `:sec` | ConfigMaps and secrets |
| `:cj`, `:jo`, `:hpa`, `:pdb` | CronJobs, jobs, HPAs, disruption budgets |
| `:pv`, `:pvc`, `:crd`, `:np` | Storage, CRDs, network policies |

On a fresh config, `Shift+0` through `Shift+9` are mapped to pods, workload dashboard, deployments, services, events, nodes, namespaces, PVCs, Pulse and XRay.

**Terminal caveat:** Some Windows terminals convert `Shift+number` into punctuation; if this happens, use the `:` commands instead. Existing `hotkeys.yaml` / `aliases.yaml` files are intentionally not overwritten. You can copy desired shortcuts from `internal/config/templates/` to your current K9s configuration directory, shown by `k9s-dev.exe info`.

## Phase 2 — Health Cockpit and advanced log exploration

Phase 2 enhances the existing **terminal interface**, without requiring a
separate dashboard server, Kubernetes custom resources, or new RBAC permissions.

### Health Cockpit

Open the existing Pulse resource dashboard with `:pulse` (or `Shift+8` when
the first-run hotkeys are installed).

- The border summary shows **HEALTHY / ATTENTION / SCANNING**, healthy resources,
  unhealthy resources, and the number of resource types sampled so far.
- The existing Pulse gauges and CPU/memory charts remain available and continue
  updating from their existing watchers.
- Press **`e`** to drill down to the resource type with the most unhealthy
  objects in the latest snapshot, or use the arrow keys and **Enter** to inspect
  any chart as before.
- **SCANNING** means not all visible resource types have reported yet. An
  empty or incomplete snapshot is never treated as fully healthy.
- Counts cover the resource *categories displayed by Pulse* and can include
  different Kubernetes objects associated with the same workload; this is not
  a count of independent applications.

The Health Cockpit **reuses Pulse data**; it does not trigger additional API calls.
If a role cannot list particular resource types, the summary should not be
interpreted as proof of cluster-wide health.

### Log Explorer

Open logs using the normal K9s keys (`l` on a selected pod) or the existing
workload log views. The indicator now has a second line describing the active
explorer state and shortcuts:

| Key | Behavior |
|---|---|
| **Shift+E** | Filter buffered and incoming lines to ERROR, FATAL, PANIC, SEVERE, CRITICAL |
| **Shift+W** | Filter to WARN or WARNING |
| **Shift+N** | Clear quick severity mode, restoring your previous manual `/` regex |
| **Shift+J** | Toggle pretty-printing for complete one-line JSON objects/arrays |
| **/** | Existing manual case-insensitive regex search (unchanged) |
| **a**, **s**, **t**, **w**, **f**, **Ctrl+S** | Existing all-container, autoscroll, timestamp, wrap, fullscreen and export actions |

Pressing the same severity shortcut again also toggles that filter off. The
severity rules match **whole words** case-insensitively. JSON formatting is
**opt-in** and applies only to complete JSON records starting with `{` or
`[`. Kubernetes timestamps, container prefixes, ANSI formatting and ordinary
text remain untouched. JSON indentation is a presentation change only; the
stored log stream is not modified.

The log explorer reuses K9s's existing bounded log buffer, regex filtering,
streaming and export features. It does not ship logs to an external service.

### CI validation

The Windows pipeline runs command tests, configuration tests, and the existing
TUI regression suite (`go test ./cmd ./internal/config ./internal/view`).
Only a successful run publishes the `k9s-dev-windows-amd64` artifact.

## Emerald Operations — redesigned default Pods screen

The **Pods screen itself** now opens as an operations cockpit. This is a
real terminal UI, not a decorative screenshot:

- A **compact header** replaces the large ASCII logo only while the cockpit
  is displayed; other K9s resource pages retain their normal header.
- A **keyboard-operated navigation rail** offers Pods, Deployments, Services,
  Events, Nodes, Namespaces, Helm, Pulse, XRay and more.
- Five counters above the **real K9s pods table** show visible pods, ready,
  issues, completed and restarts. These are derived from the visible table
  rows, *not* total Kubernetes cluster inventory. Completed Jobs are not
  counted as unhealthy.
- A lower **three-column investigation pane** follows the highlighted pod:
  Pod Doctor, recent container logs, and UID-filtered pod Events.
- Selecting a different pod automatically refreshes the lower panes after a
  short debounce; in-flight requests for the previous selection are cancelled.
- Native K9s shortcuts, filters, sorting, logs, shell, YAML, descriptions and
  plugins remain in the existing table.

| Keys | Action |
|---|---|
| `Shift+B` | Toggle full-table K9s view versus Emerald Operations cockpit |
| `Ctrl+N` | Focus the resource navigation rail |
| `Up/Down` then `Enter` in navigation | Open the chosen native resource view |
| `Esc` or `Tab` in navigation | Return keyboard focus to the table |
| `Shift+V` | Open the dedicated full-page Pod Workspace |
| `Shift+D` | Open the dedicated full-page Pod Doctor |
| `Ctrl+E` | Toggle the header |

The dashboard is visible by default when entering `:pods`, including at
startup when Pods is the configured starting page. Switching to other resource
views automatically hides the extra dashboard panels. The temporary selection
logs are *bounded on-demand snapshots*, not long-running watch streams; use
`l` for real-time streaming.

A screenshot of a UI concept is **not** a promise of rounded buttons, neon glow,
or embedded browser controls: terminals provide borders, text styling, lists,
real tables and keyboard/mouse navigation. The navigation list is functional,
and the same Kubernetes authentication/RBAC rules apply as before.

## Phase 4 — Interactive Recent Logs

The **main Emerald Pods cockpit** now has a selectable Recent Logs table and
a toggleable whole-pane beautified view. See the full
[Phase 4 implementation specification](PHASE4_LOG_EXPLORER.md).

**Navigation:** Tab and Shift+Tab cycle Sidebar, Pods, Pod Doctor, Recent Logs,
and Pod Events. The old Ctrl+N shortcut is no longer needed. Use the arrows
inside the focused pane. Enter on a selected log opens a detailed formatted
view with the original raw line; Esc returns.

**Recent Logs shortcuts:**

| Shortcut | Effect |
|---|---|
| Tab / Shift+Tab | Move between cockpit panels |
| Up / Down and Enter | Select a log and open its detailed beautified view |
| b | Switch compact table / whole-pane formatted logs |
| / | Focus the case-insensitive search field (Enter keeps, Escape clears) |
| a / e / w / i / d | Filter all / errors / warnings / info / debug |
| [ / ] | Decrease/increase fetched lines (20–500) |
| , / . | Decrease/increase maximum fetched bytes (64 KiB–1 MiB) |
| + / - | Enlarge/shrink the bottom diagnostic panels |
| r | Reload the selected pod using active settings |
| W | Toggle wrapping in formatted view |
| c / C | Copy original / beautified selected log entry |
| Ctrl+S | Explicitly export visible raw lines to the K9s screen-dump folder |
| g / G, n / N | Top/bottom and next/previous matching entry |

The compact Recent Logs pane displays TIME, LEVEL and MESSAGE. JSON log lines
are parsed even when preceded by a Kubernetes timestamp. Both modes retain
unstructured messages and the original raw text. All application log text is
escaped before rendering as terminal colors. **This is a bounded snapshot,
not live streaming**; use the original K9s log viewer for live streams.

Log settings currently persist for the **current session**. Exported files may
contain sensitive application information; review and redact before sharing.

## Phase 3 — Pod Workspace and Pod Doctor (terminal UI)

Select a pod in `:pods` and press **Shift+V** to open the **Pod Workspace**.
Press **Shift+D** to open the same view focused on **Pod Doctor**.

The new viewer uses ordinary tview/tcell terminal primitives: a green
header/footer and three scrollable bordered panes. It is **not** a separate
desktop interface, so all existing Kubernetes viewers continue to work.

| Key in the workspace | Action |
|---|---|
| `Tab` / `Shift+Tab` | Move focus between diagnostics, recent logs and pod events |
| `r` | Refresh the three read-only snapshots |
| `l` | Open the existing full, streaming K9s log viewer |
| `e` | Open the normal namespace Events viewer |
| `Esc` or `q` | Return to the pods table |
| `Ctrl+E` | Existing global K9s toggle to hide the large header |

**Pod Doctor** detects CrashLoopBackOff, previous OOMKilled termination,
non-zero exits, image-pull failures, missing configuration, repeated restarts,
unscheduled pods, not-ready pods and warning events. For each finding it
suggests an investigation; it does **not** restart or modify workloads.

The workspace reads the selected pod with Kubernetes GET; it attempts a
namespace-scoped Events list filtered by pod name/UID, and a **bounded**
snapshot of up to 80 recent log lines (128 KiB) for the pod's default or first
regular container. When the container is in CrashLoopBackOff and has restarted,
the preview requests previous-container logs. Log/event RBAC failures appear
in their own panes rather than hiding the other diagnostic information.

**Limitations:** the preview is an on-demand snapshot, not a continuous
stream; press `r` to reload or `l` for live logs. The event pane shows up to
20 events from the first 100 API-matched events; it may not contain every
historical event. Pod Doctor findings are evidence-based hints, not automatic
root-cause determinations. No secrets or environment variables are fetched.

## Cluster Doctor

Run:

```powershell
.\k9s-dev.exe doctor
.\k9s-dev.exe doctor --context my-context --timeout 12s
.\k9s-dev.exe doctor --kubeconfig C:\Users\You\.kube\config
```

The command checks local context selection, Kubernetes API connectivity and your current identity's effective permissions for **listing pods, reading logs, exec into pods, listing nodes and deleting pods**. Checks use the Kubernetes `SelfSubjectAccessReview` API and do **not** perform those privileged operations. If the API refuses permission probes, status is shown as unknown rather than incorrectly calling them denied. No tokens or certificates are printed.

## Build locally (optional)

Install the Go toolchain version required by `go.mod`, then from the repository root:

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o .\k9s-dev.exe .
```

Or build from Linux/WSL:

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o k9s-dev.exe .
```

The GitHub Actions workflow builds the exe automatically on pushes to the feature branch and `main`, as well as when manually dispatched from a branch where the workflow is available. The artifact is downloadable only **after a successful workflow run**; no binary is checked into git.

## Safety and compatibility

- Existing K9s features such as logs, shell, context selection, port forwarding, resource inspection, helm charts, XRay and Pulse are retained.
- Cluster operations use the same kubeconfig, Kubernetes authentication and RBAC as upstream K9s.
- Operator-configured skins, aliases and hotkeys are preserved.
- No executable or plugin is downloaded at program startup.
- The original upstream K9s Apache-2.0 license and attribution remain intact.
