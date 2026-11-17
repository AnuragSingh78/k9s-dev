# K9s Dev — Emerald Edition

This build keeps the existing [K9s](https://k9scli.io/) terminal application and its Kubernetes features. It adds an **embedded emerald/forest-green UI**, keyboard navigation, resource aliases, a **read-only cluster doctor**, and an automated **Windows x64 `.exe` build**.

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
