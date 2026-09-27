# Lars-Win-AI — Eingefrorene Verträge (Phase 0)

> **Status: FROZEN.** Dieses Dokument ist die maßgebliche Schnittstelle zwischen den
> parallelen Workstreams (Phase 1). Änderungen nur durch den Orchestrator.
> Alle Workstreams MÜSSEN diese Namen, Pfade und Schemata exakt einhalten.

Sprache: Antworten/Dokumentation **Deutsch**, Code-Bezeichner/Identifier Englisch.

---

## 1. Repository-Layout (verbindlich)

```
Lars-Win-AI/
├─ README.md                      # WS-CI
├─ AGENTS.md                      # Phase 0 (Orchestrator)
├─ PLAN.md                        # vorhanden
├─ manifest.json                  # Phase 0
├─ manifest.schema.json           # Phase 0
├─ go.mod / go.sum                # Phase 0 (go.mod) + WS-D
├─ install.ps1                    # WS-I
├─ uninstall.ps1                  # WS-I
├─ docs/
│  ├─ CONTRACTS.md                # Phase 0 (diese Datei)
│  └─ TESTING.md                  # WS-CI
├─ scripts/
│  ├─ lib.ps1                     # WS-I
│  └─ components/
│     ├─ deps.ps1                 # WS-I
│     ├─ alacritty.ps1            # WS-I
│     ├─ herdr.ps1                # WS-I
│     ├─ neovim.ps1               # WS-I
│     ├─ opencode.ps1             # WS-I
│     └─ daemon.ps1               # WS-I
├─ config/
│  ├─ alacritty/alacritty.toml    # WS-A
│  ├─ herdr/config.toml           # WS-A
│  ├─ nvim/**                     # WS-N
│  └─ opencode/
│     ├─ themes/ai-transparent.json   # WS-O
│     ├─ mcp.snippet.jsonc            # WS-O
│     ├─ skill/SKILL.md               # WS-O (herdr-Skill-Kopie)
│     └─ README.md                    # WS-O (Merge-Erwartung)
├─ cmd/aid/**                     # WS-D
├─ internal/**                    # WS-D
├─ tests/
│  ├─ pester/**                   # WS-I
│  ├─ e2e/**                      # WS-I
│  └─ ci/**                       # WS-CI
└─ .github/workflows/
   ├─ ci.yml                      # WS-CI
   └─ release.yml                 # WS-CI
```

**Dateibesitz (kein Überschreiben fremder Dateien):**

| Workstream | Eigene Pfade |
|---|---|
| WS-D | `cmd/**`, `internal/**`, `go.mod`, `go.sum` |
| WS-A | `config/alacritty/**`, `config/herdr/**` |
| WS-N | `config/nvim/**` |
| WS-O | `config/opencode/**` |
| WS-I | `install.ps1`, `uninstall.ps1`, `scripts/**`, `tests/pester/**`, `tests/e2e/**` |
| WS-CI | `README.md`, `.github/**`, `docs/TESTING.md`, `tests/ci/**` |

---

## 2. `manifest.json` — Schema

Vollständiges Schema: `manifest.schema.json`. Struktur:

```jsonc
{
  "name": "Lars-Win-AI",
  "version": "0.1.0",
  "components": [
    {
      "id": "deps",                     // lowercase, [a-z]+
      "function": "Install-Deps",       // MUSS Install-<PascalId> sein
      "script": "scripts/components/deps.ps1",
      "order": 10,                      // aufsteigend = Ausführungsreihenfolge
      "default": true,                  // bei -Yes ohne -Components aktiv
      "requires": [],                   // Komponenten-IDs (müssen order < haben)
      "description": "…"
    }
  ],
  "symlinks": [
    {
      "id": "alacritty-config",
      "kind": "junction",               // "junction" | "symlink"
      "source": "config/alacritty",     // relativ zum Repo-Root
      "target": "%APPDATA%/alacritty",  // %VAR% wird expandiert; / ist ok
      "fallback": "copy",               // "copy" | "error"  (bei symlink)
      "backup": true,                   // vorhandenes Ziel sichern → *.bak.<ts>
      "component": "alacritty"          // zugehörige Komponente
    }
  ],
  "env": {
    "AID_WINDOW_TITLE": "AI-Assistant",
    "AID_MONITOR": "2",
    "AID_AGENT_NAME": "opencode",
    "AID_AGENT_KIND": "opencode",
    "AID_AGENT_ARGS": "--auto"
  },
  "release": {
    "asset": "aid.exe",
    "repo": "Winkelars/Lars-Win-AI"
  }
}
```

**Komponenten-IDs und Reihenfolge (fix):**

| id | function | order | requires | default |
|---|---|---|---|---|
| deps | Install-Deps | 10 | – | true |
| alacritty | Install-Alacritty | 20 | deps | true |
| herdr | Install-Herdr | 30 | deps | true |
| neovim | Install-Neovim | 40 | deps | true |
| opencode | Install-Opencode | 50 | deps | true |
| daemon | Install-Daemon | 60 | – | true |

**Symlink-/Junction-Zieltabelle (fix):**

| id | kind | source | target | fallback | backup |
|---|---|---|---|---|---|
| alacritty-config | junction | `config/alacritty` | `%APPDATA%/alacritty` | – | true |
| nvim-config | junction | `config/nvim` | `%LOCALAPPDATA%/nvim` | – | true |
| herdr-config | symlink | `config/herdr/config.toml` | `%APPDATA%/herdr/config.toml` | copy | true |
| opencode-themes | junction | `config/opencode/themes` | `%USERPROFILE%/.config/opencode/themes` | – | true |
| opencode-skill | junction | `config/opencode/skill` | `%USERPROFILE%/.config/opencode/skills/herdr` | – | true |

> `opencode.jsonc` wird **nicht** gejunctiont, sondern per Merge (`.bak`) gepatcht.

---

## 3. Installer-Modul-Vertrag

Jede Komponente ist eine `.ps1`-Datei, die **genau eine** Funktion definiert.

```powershell
function Install-<PascalId> {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    # ... arbeitet ausschließlich über die lib.ps1-Helfer ...
    return @{
        Component = '<id>'                 # string, == manifest id
        Status    = 'installed'            # 'installed' | 'skipped' | 'failed'
        Changed   = $true                  # bool: hat etwas verändert?
        Messages  = @('…')                 # string[]
    }
}
```

**`$Context` (hashtable) — garantierte Schlüssel:**

| Key | Typ | Bedeutung |
|---|---|---|
| `RepoRoot` | string | absoluter Pfad zum Repo |
| `Manifest` | pscustomobject | geparstes `manifest.json` |
| `Config` | hashtable | geparste Daemon-Config (siehe §5) |
| `Yes` | bool | non-interaktiv |
| `DryRun` | bool | nur planen, nichts schreiben |
| `SkipDeps` | bool | winget überspringen |
| `SelectedComponents` | string[] | aktive Komponenten-IDs |
| `LogFile` | string | Pfad zum Journal (`install.log`) |
| `LibPath` | string | absoluter Pfad zu `scripts/lib.ps1` |
| `Result` | hashtable | aggregiertes Ergebnis (Installer füllt) |

Jede Komponente MUSS `scripts/lib.ps1` per Dot-Source laden (z. B.
`if (-not (Get-Command Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }`)
und darf **keine** Winget-/Dateisystem-Aktion außerhalb der lib-Helfer ausführen,
sofern ein Helfer existiert.

**Gesamt-Ergebnis (Installer, JSON-ausgebbar):**

```jsonc
{
  "ok": true,
  "dryRun": false,
  "startedAt": "2026-…Z",
  "finishedAt": "2026-…Z",
  "components": [ { "component": "deps", "status": "installed", "changed": true, "messages": [] } ],
  "env": { "applied": ["EDITOR", "VISUAL"] },
  "warnings": []
}
```

**Installer-Parameter (fix):** `-Components <string[]>`, `-Yes`, `-DryRun`,
`-SkipDeps`, `-Config <path>`.
**Exitcodes:** `0` Erfolg, `1` Fehler, `2` Preflight fehlgeschlagen.
`-DryRun`/`-WhatIf` schreibt **nichts** (kein Journal, keine Backups).

---

## 4. `scripts/lib.ps1` — Helper-Vertrag (fix)

```powershell
function Write-Log        { param([string]$Message,[ValidateSet('Info','Warn','Error','Success','Debug')][string]$Level='Info',[string]$LogFile) }
function Test-Command     { param([string]$Name) }                            # -> [bool]
function Test-Admin       { }                                                 # -> [bool]
function Test-DeveloperMode { }                                               # -> [bool]
function Resolve-ManifestPath { param([string]$Path) }                        # %VAR%/~/ -> absolut
function Backup-Path      { param([string]$Path) }                            # -> [string]|$null (Backup-Pfad)
function New-JunctionSafe { param([string]$LinkPath,[string]$TargetPath,[switch]$DryRun,[switch]$Force) }  # -> [hashtable]@{Ok;Changed;Mode}
function New-FileSymlinkOrCopy { param([string]$LinkPath,[string]$TargetPath,[switch]$DryRun,[switch]$Force) } # -> @{Ok;Changed;Mode='symlink'|'copy'}
function Merge-OpencodeJsonc { param([string]$Path,[hashtable]$Fragment,[switch]$DryRun) } # -> @{Ok;Changed;Backup;Mode='jsonc'|'json'}
function Invoke-Winget    { param([string]$Id,[string]$Name,[switch]$DryRun,[switch]$SkipDeps) } # -> @{Ok;Installed;Changed}
function New-AidResult    { param([string]$Component) }                       # -> leeres Komponenten-Ergebnis (§3)
```

Regeln:
- **Idempotent:** läuft N-mal gefahrlos; erkennt existierende Junctions/Symlinks
  über `(Get-Item -Force).LinkType`/`Target` und überspringt, was schon stimmt.
- **Junctions** brauchen kein Admin; **File-Symlinks** brauchen Dev-Mode/Admin →
  `New-FileSymlinkOrCopy` fällt bei Fehler auf Copy zurück (`Mode='copy'`).
- **JSONC-Merge** (`Merge-OpencodeJsonc`) erhält Kommentare, fügt unter `mcp`
  fehlende Keys hinzu (kein Clobber vorhandener `exa`/`zoho`/`playwright`/`brain`),
  und schreibt vorher `Backup-Path`.
- Jede schreibende Aktion respektiert `-DryRun`.

---

## 5. Daemon: Config-Schema + CLI (fix)

**Config-Datei:** `%APPDATA%\Lars-Win-AI\config.json` (Override: `--config`,
Env: `AID_CONFIG`). Alle Felder optional; Defaults greifen.

```jsonc
{
  "window_title": "AI-Assistant",         // Env AID_WINDOW_TITLE
  "process_name": "alacritty.exe",
  "monitor": 2,                           // Env AID_MONITOR (1-basiert)
  "agent_name": "opencode",               // Env AID_AGENT_NAME
  "agent_kind": "opencode",               // Env AID_AGENT_KIND
  "agent_args": ["--auto"],               // Env AID_AGENT_ARGS (space-split)
  "alacritty_path": "C:\\Program Files\\Alacritty\\alacritty.exe",  // Env AID_ALACRITTY_PATH
  "alacritty_config": "<repo>/config/alacritty/alacritty.toml",      // Env AID_ALACRITTY_CONFIG
  "herdr_path": "herdr",                  // Env AID_HERDR_PATH (testbar/stubbar)
  "pane_state_file": "%APPDATA%\\Lars-Win-AI\\pane-state.json",
  "hotkey": { "scan_code": 41, "alt_only": true, "exclude_altgr": true },
  "animations": true,
  "startup_timeout_ms": 8000,
  "herdr_retry_ms": 500,
  "herdr_retry_count": 40
}
```

> ScanCode 0x29 = 41 dezimal (US-Layout `^`, layoutunabhängig).

**CLI (fix):**

```
aid run       [--config PATH] [--no-hook]      # Standard: Hook + Toggle-Loop
aid register  [--config PATH]                  # Task Scheduler "At log on"
aid unregister
aid status    [--config PATH] [--json]
aid toggle    [--config PATH] [--no-hook]      # ein einzelner Toggle (Test/Debug)
aid version | --version
aid help | --help
```

- `--no-hook`: Hook wird nicht installiert; erlaubt Test/VM-Betrieb.
- `register` legt Task `Lars-Win-AI\\aid` mit Trigger `At log on`, Settings
  `-RunLevel Highest` ist NICHT nötig (Hook braucht kein Admin),
  `-ExecutionTimeLimit 0`, `-RestartCount 3`, Action = `<repo>\bin\aid.exe run`.
- Build: `go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid`
  (**keine** Konsolenausgabe im Hintergrund).

**Env-Variablen (fix, vom Installer gesetzt):** `AID_WINDOW_TITLE`,
`AID_MONITOR`, `AID_AGENT_NAME`, `AID_AGENT_KIND`, `AID_AGENT_ARGS`,
`AID_ALACRITTY_PATH`, `AID_HERDR_PATH`.

---

## 6. Daemon: Go-Paketstruktur + interne Interfaces (fix)

```
cmd/aid/main.go                # CLI-Dispatch, Signal-Handling, windowsgui
internal/config/               # Load, Defaults, Env-Overrides, Validate (+ _test)
internal/logging/              # Datei-Logger (%LOCALAPPDATA%\Lars-Win-AI\aid.log)
internal/window/               # Fenster-Steuerung hinter Interface (+ _test)
internal/hotkey/               # WH_KEYBOARD_LL Hook hinter Interface (+ _test)
internal/herdr/                # CLI-Bridge, JSON-Parse, Stub-Typen (+ _test)
internal/toggle/               # Zustandsmaschine, pur, 100% testbar (+ _test)
internal/platform/             # build-tagged Windows-Syscalls, Stubs für !windows
```

**Zwingende Interfaces (Namen fix):**

```go
// internal/window
type Manager interface {
    FindByProcess(processName string) (Window, bool)
    FindByTitle(title string) (Window, bool)
    Enumerate() ([]Window, error)
}
type Window interface {
    Handle() uintptr
    Title() string
    ProcessName() string
    IsForeground() bool
    Restore() error
    Minimize() error
    ShowNormal() error
    Focus() error
    MoveAndMaximize(monitor int) error
}

// internal/herdr
type Client interface {
    AgentList(ctx context.Context) ([]Agent, error)
    AgentFocus(ctx context.Context, name string) error
    AgentStart(ctx context.Context, name, kind, paneID string, args []string) error
    PaneList(ctx context.Context) ([]Pane, error)
    PaneSplit(ctx context.Context, paneID, direction string) (string, error)
    WorkspaceFocus(ctx context.Context, workspaceID string) error
    TabFocus(ctx context.Context, tabID string) error
}
type Agent struct { Label, Name, Kind, Status, PaneID, TabID, WorkspaceID string; Focused bool }
type Pane  struct { ID, TabID, WorkspaceID, CWD string; AgentName string; Focused bool }

// internal/toggle
type Action int // ActionNone, ActionStart, ActionForeground, ActionFocusPane, ActionMinimize, ActionFocusAndForeground
type Deps struct {
    Windows window.Manager
    Herdr   herdr.Client
    CFG     *config.Config
}
func Decide(foreground bool, windowExists bool, opencodeFocused bool) Action
func Run(ctx context.Context, d Deps, cfg *config.Config) (Action, error)
```

**Zustandsmaschine `Decide` (pur, testbar):**

| windowExists | foreground | opencodeFocused | Action |
|---|---|---|---|
| false | – | – | `ActionStart` |
| true | false | – | `ActionForeground` |
| true | true | false | `ActionFocusPane` |
| true | true | true | `ActionMinimize` |

> `opencodeFocused` wird primaer aus `herdr agent list` (`focused`) und mit
> Fallback ueber `herdr pane list` (`focused` des Agent-Panes) bestimmt.
> `ActionStart`/`ActionForeground` fokussieren zusaetzlich das opencode-Pane
> (`focusOpencode`).
>
> `MoveAndMaximize` legt das Fenster randlos ueber den **Arbeitsbereich** des
> Zielmonitors (windowed, ausgebreitet; Taskleiste bleibt sichtbar) — kein
> Vollbild.

**Herdr-Bridge-Regeln:**
- `herdr agent list` parsen: Agents liegen unter `.result.agents[]` mit Feldern
  `agent`, `agent_status`, `focused`, `pane_id`, `tab_id`, `workspace_id`.
- opencode-Agent finden: primaer ueber den eindeutigen `name`, sonst ueber das
  Label/Kind (`agent`); Agenten ohne Namen werden ueber ihre `pane_id` fokussiert.
- Fokus-Sequenz (wichtig): `workspace focus <ws>` -> `tab focus <tab>` ->
  `agent focus <pane|name>`. Ein Pane-Fokus zieht Tab/Workspace **nicht**
  automatisch mit; ohne die Sequenz bleibt der sichtbare View stehen.
- Kein Agent → designierten Pane aus `pane_state_file` lesen, mit `pane list`
  validieren; sonst `pane split <pane> --direction right --no-focus` →
  neue Pane-ID aus `.result.pane.pane_id`; dann
  `agent start <name> --kind <kind> --pane <id> -- <args...>`.
- Fehlender Server/Socket → `herdr_retry_count` × `herdr_retry_ms` warten.
- Alles außerhalb von `internal/platform` muss auf Nicht-Windows kompilieren
  (`go build ./...` auf linux/windows identical), daher build-tags.

**Testing-Anforderung:** `internal/toggle`, `internal/config`, `internal/herdr`
(JSON-Parse + Stub-Client) und `internal/window` (Fake-Manager) brauchen
`_test.go` mit `go test ./...` grün. Die echten Windows-Syscalls
(`golang.org/x/sys` NICHT verwenden → **nur Go-Standardbibliothek**).

---

## 7. Alacritty-Asset-Vertrag (WS-A)

- Datei: `config/alacritty/alacritty.toml`, Alacritty ≥ 0.13 TOML-Format.
- Pflichtwerte:
  - `[window] decorations = "None"`, `opacity = 0.90`, `dynamic_title = true`,
    `title = "AI-Assistant"`, `startup_mode = "Windowed"`, `padding = { x = 8, y = 8 }`,
    `dynamic_padding = true`, `blur = false` (Windows-Fallback).
  - `[font] family = "CaskaydiaCove Nerd Font"`, `size = 15.0`, `normal.family`,
    `bold/italic/bold_italic` ebenfalls CaskaydiaCove.
  - `[keyboard] bindings`: `{ key = "Space", mods = "Control|Shift", action = "ToggleViMode" }`.
  - `[terminal] shell = { program = "herdr" }` — **oder** Start via `-e herdr`;
    Empfehlung: `-e herdr` beim Start (Installer/Daemon), damit Terminal-Sessions
    ohne Herdr nicht erzwungen werden.
- Startzeile (Daemon/Installer):
  `alacritty.exe -T "AI-Assistant" --config-file <repo>\config\alacritty\alacritty.toml -e herdr`
- Hintergrund/Transparenz: `[colors.primary] background = "#000000"` und
  `[window] opacity`; opencode-Theme liefert Panel-Transparenz.

## 8. Herdr-Asset-Vertrag (WS-A)

- Datei: `config/herdr/config.toml`. Bestehende `[ui] host_cursor = "native"`
  beibehalten, ergänzen:
  - `[ui] window_title = "AI-Assistant"` (stabiles OS-Fenstertitel zum Auffinden).
  - Theme-Block mit transparentem Panel-Hintergrund passend zu Alacritty
    (`opacity`/`transparent`-Optionen, je nach Herdr-Theme-Schema).
- `Herdr`-Integration: Installer (WS-I) führt
  `herdr integration install opencode` aus; WS-A liefert nur die Config.

## 9. LazyVim-Asset-Vertrag (WS-N)

- Ordner `config/nvim/` == 1:1-Kopie der bestehenden Config
  (`C:\Users\Makkis\AppData\Local\nvim`, LazyVim-Starter, git-Repo), OHNE `.git`.
- Dateien: `init.lua`, `lua/config/{options,keymaps,autocmds,lazy}.lua`,
  `lua/plugins/tokyonight.lua`, `lazy-lock.json`, `lazyvim.json`,
  `stylua.toml`, `.neoconf.json`, `.gitignore`, `README.md` (LazyVim).
- **Markdown-Diagnostics aus:** in `lua/plugins/lazyvim.lua` (neu) oder
  `lua/config/autocmds.lua` ein `LspAttach`-Autocmd, das bei `filetype=markdown`
  `vim.diagnostic.enable(false, { bufnr = bufnr })` setzt — `<leader>ud` bleibt
  als Toggle nutzbar.
- **Spell:** `vim.opt.spelllang = { "en", "de" }` (in `options.lua`).
- `lazy-lock.json` unverändert übernehmen (reproduzierbare Versionen).

## 10. opencode-Asset-Vertrag (WS-O)

- Theme: `config/opencode/themes/ai-transparent.json`. Schema wie
  opencode-Theme (top-level `"$schema"`, Def `background`, `backgroundPanel`,
  `backgroundElement`, `backgroundMenu` je `"none"`). Aktivierung später via
  `"theme": "ai-transparent"` (Installer-Merge).
- MCP-Snippet: `config/opencode/mcp.snippet.jsonc` mit **exa** (remote,
  `https://mcp.exa.ai/mcp`) und **playwright** (local,
  `["cmd","/c","playwright-mcp","--browser","chromium"]`).
  Installer merged dies idempotent in `~/.config/opencode/opencode.jsonc`.
- Skill: `config/opencode/skill/SKILL.md` = Kopie von `herdr --skill`
  (Frontmatter + Body), damit der Installer ihn als
  `~/.config/opencode/skills/herdr/SKILL.md` installieren kann.
- `config/opencode/README.md`: dokumentiert erwartetes Merge-Verhalten
  (kein Clobber von `zoho`/`brain`, Kommentare erhalten, `.bak`).

## 11. CI/Docs-Vertrag (WS-CI)

- `.github/workflows/ci.yml` auf `windows-latest`, Trigger `push`/`pull_request`:
  1. Go: `go vet ./...`, `go test ./...`, Build `windows/amd64`.
  2. PowerShell: PSScriptAnalyzer über `install.ps1`, `uninstall.ps1`,
     `scripts/**`, `tests/**`.
  3. Pester: `Invoke-Pester tests/pester`.
  4. Config-Validierung (ohne Systemänderung): JSON (`manifest.json`,
     Theme), TOML (Alacritty, Herdr), Lua-Syntax (LazyVim), `install.ps1 -DryRun`.
- `.github/workflows/release.yml` auf Tag `v*`: Build `windows/amd64`
  `-ldflags "-H=windowsgui -s -w"`, Upload `bin/aid.exe` als Release-Asset
  `aid.exe` (Repo `Winkelars/Lars-Win-AI`).
- `.github/workflows/release.yml` Artefakt-Name **exakt** `aid.exe`.
- `docs/TESTING.md`: Layer 1–3 + Testmatrix aus `PLAN.md` §11 zusammenfassen.

---

## 12. Herdr-Orchestrierung (Kontext für alle Agents)

- Referenz: **`herdr --skill`** (maßgeblich). `HERDR_ENV=1` prüfen.
- Orchestrator: dieser Tab bleibt exklusiv; Subagenten laufen in **einem neuen
  Tab** in **Split-Panes**.
- IDs aus JSON lesen (`.result.*`), nicht raten.
- Agent-Slots: `herdr agent start <name> --kind opencode --pane <id> -- --auto`;
  Arbeit per `herdr agent prompt <name> "<text>" --wait`; Status per
  `herdr agent list` / `herdr agent read <name>`.

## 13. Git-Regeln (alle Workstreams)

- **Nicht committen** — Änderungen im Working Tree lassen. Der Orchestrator
  committet ggf. nach Integration.
- Keine Secrets in Dateien.
- Keine `.exe`/Build-Artefakte einchecken (siehe `.gitignore`).
