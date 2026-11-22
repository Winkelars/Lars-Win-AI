# Lars-Win-AI — Dream-Workflow für AI-Assistenten unter Windows

Ziel: Ein GitHub-Repo, das den kompletten Windows-Workflow um einen
AI-Assistenten **kontrolliert** installiert und reproduzierbar macht. Ersetzt
AutoHotkey als Abhängigkeit durch einen eigenen Hintergrundprozess.

## 1. Zielsetup

- **Hotkey `Alt + ^` (Zirkumflex)** öffnet / minimiert / fokussiert das
  AI-Assistenten-Fenster je nach Ursprungszustand (wie das bisherige
  AHK-Skript, siehe Referenz `Quick Apps.ahk`).
- Das Fenster ist effektiv **eine WezTerm-Instanz**; darin läuft **Herdr**.
- Beim Beenden von Herdr schließt sich das WezTerm-Fenster.
- Herdr dient auch für andere PowerShell-Sitzungen. Der Hotkey soll aber gezielt
  das **designierte opencode-Panel** fokussieren.
- Arbeiter der User bereits in Herdr, soll der Hotkey **schnell direkt zum
  opencode-Panel wechseln**.
- **Neovim (LazyVim)** als Editor in opencode:
  - Markdown-Dateien: Diagnostics per Default **aus** (`<leader>ud` zum Togglen).
  - Spellchecking **Deutsch + Englisch**.
- **WezTerm**:
  - Font **„CaskaydiaCove Nerd Font"** (über Repo automatisch bereitstellbar).
  - `ctrl+shift+space` (Visual Mode / Copy) = Standardwert, essenziell.
  - Windows-native Titelleiste **ausgeblendet** (nackt).
  - Leicht transparent (Wert offen, zentral konfigurierbar).
- **opencode**: Theme mit **100 % transparentem Hintergrund**, falls möglich.
- **`herdr --skill`** wird als opencode-Skill installiert.
- **Playwright-MCP** und **Exa Search MCP** werden konfiguriert.
- Die opencode-Sitzung wird standardmäßig mit **`--auto`** gestartet
  (Auto-Approve nicht explizit verneinter Permissions).

## 2. Ist-Zustand (verifiziert auf dem Referenzsystem)

| Komponente | Status | Detail |
|---|---|---|
| AHK-Skript | aktiv | `Startup\Quick Apps.ahk`; `!^` toggelt heute das **DeepSeek**-Chromium-Fenster (nicht WezTerm) |
| WezTerm | installiert (winget), **keine Config** | `C:\Program Files\WezTerm\wezterm.exe`, Config-Default `~/.wezterm.lua` |
| Herdr | installiert, Server läuft | 0.9.0-preview, Socket `%APPDATA%\herdr\herdr.sock`, Config `%APPDATA%\herdr\config.toml` |
| opencode | installiert (scoop) | 1.18.32; Config `~/.config/opencode/`; enthält herdr-managed `tui.jsonc`, `herdr-tui-session.js`, `plugins/herdr-agent-state.js` |
| Neovim/LazyVim | installiert | `%LOCALAPPDATA%\nvim` ist bereits ein **git-Repo** (LazyVim-Starter) |
| CaskaydiaCove Nerd Font | **bereits installiert** | winget `ryanoasis.CaskaydiaCove` 3.4.0 → kein Bundling nötig |
| MCP (exa, playwright) | bereits in `opencode.jsonc` | zoho & brain-reference ebenfalls vorhanden |

Vault-Konventionen (relevant): Antworten Deutsch, Stack **Go** bevorzugt,
winget/scoop bevorzugt, `mcp`-Key (nicht `mcpServers`).

## 3. Zielarchitektur

```
┌─ Task Scheduler (Logon) ─────────────────────────────┐
│  aid - Go-Daemon, kein Konsolenfenster               │
│   • WH_KEYBOARD_LL-Hook: Alt+^ (layoutunabhängig,    │
│     ScanCode 0x29, Alt-only, AltGr ausgeschlossen)   │
│   • Fenster: finden/starten/fokussieren/minimieren,  │
│     Move+Maximize auf Monitor 2, Animationen aus     │
│   • Herdr-Bridge: agent list → focus/start opencode  │
└───────────────┬──────────────────────────────────────┘
                │ startet/spricht
        WezTerm (borderless, CaskaydiaCove, opacity,
        ctrl+shift+space vi-mode)  start -- herdr
                │
        Herdr-Server (persistente Panes)
                ├─ Pane: opencode  ← designiertes Pane
                └─ Pane: PowerShell-Sessions
                        opencode → Editor nvim, Theme transparent,
                                   MCP exa+playwright, Skill herdr
```

### Toggle-Zustandsmaschine (Alt+^)

1. Fenster existiert nicht → WezTerm + `herdr` starten → auf Monitor 2 in den
   Arbeitsbereich legen (windowed, ausgebreitet) und fokussieren → opencode-Pane
   fokussieren (ggf. starten).
2. Fenster existiert, nicht Vordergrund → nach vorn holen (restore), ggf. in den
   Arbeitsbereich von Monitor 2 legen → opencode-Pane fokussieren.
3. Fenster Vordergrund, opencode-Pane **nicht** fokussiert → nur opencode-Pane
   fokussieren (wechselt ggf. in dessen Tab/Workspace).
4. Fenster Vordergrund und opencode-Pane fokussiert → **verstecken**
   (`SW_HIDE`, nicht `SW_MINIMIZE`; echtes Minimieren setzt die ConPTY-Groesse
   auf 0 und beendet die opencode-Session).

### opencode-Pane-Auflösung

`herdr agent list` → Agent `kind=opencode` vorhanden → `herdr agent focus <name>`.
Nicht vorhanden → designierten Pane (persistiert in State-Datei, validiert)
nutzen, sonst `herdr pane split` + `herdr agent start <name> --kind opencode
--pane <id> -- --auto`. Herdr reicht Agent-Argumente nach `--` durch
(verifiziert mit `herdr 0.9.0-preview`), `--auto` ist damit sauber übergebbar.

## 4. Entschiedene Design-Fragen

| Frage | Entscheidung |
|---|---|
| Daemon-Sprache | **Go** (eine statische `.exe`, keine Runtime) |
| Config-Verwaltung | **Symlink/Junction** (Repo = Single Source of Truth) |
| Daemon-Auslieferung | **GitHub-Actions-Release**, Installer lädt `aid.exe` |
| Herdr-Fokus + opencode-Pane nicht fokussiert | **Zum opencode-Panel springen** (nicht minimieren) |
| Keine opencode-Session | **opencode-Session starten** |
| Autostart | **Task Scheduler** (At log on) |
| opencode-Aufruf | Standardmäßig mit **`--auto`** (Auto-Approve) |

## 5. Repo-Struktur

```
Lars-Win-AI/
├─ README.md                     # Setup, Voraussetzungen, Troubleshooting
├─ AGENTS.md                     # Konventionen für künftige Agents
├─ PLAN.md                       # Diese Datei
├─ manifest.json                 # Komponenten, Reihenfolge, Zielpfade, Symlink-Map
├─ install.ps1                   # Bootstrap/Orchestrator (interaktiv, idempotent, Journal)
├─ uninstall.ps1
├─ scripts/
│  ├─ lib.ps1                    # Logging, Symlink/Junction, Backup, Merge-Helfer
│  └─ components/                # je eine Install-Funktion
│     ├─ deps.ps1   (winget: herdr, wezterm, neovim)
│     ├─ wezterm.ps1
│     ├─ herdr.ps1
│     ├─ neovim.ps1
│     ├─ opencode.ps1
│     └─ daemon.ps1              # Release laden, Task registrieren
├─ config/
│  ├─ wezterm/wezterm.lua
│  ├─ herdr/config.toml
│  ├─ nvim/{init.lua,lua/config/*,lua/plugins/*,lazyvim.json,lazy-lock.json}
│  ├─ opencode/{theme json, mcp snippet, skill}
│  └─ crush/{skill, README}
├─ cmd/aid/                      # Go-Daemon (main, hook, window, herdr, config)
├─ internal/...                  # Go-Pakete + _test.go
├─ .github/workflows/release.yml # Build → Release-Asset aid.exe
└─ tests/                        # Pester (Installer) + Go-Tests
```

## 6. Komponenten im Detail

### A) Go-Daemon (`aid.exe`)

- Globaler Low-Level-Hook (`SetWindowsHookEx(WH_KEYBOARD_LL)`),
  Tastenerkennung über **ScanCode** (layoutunabhängig), Alt links/rechts
  unterscheiden; Alt+^ wird konsumiert und nicht weitergereicht.
- Fensterhandling via `user32`/`gdi32`: `EnumWindows` + Titel/Prozess
  `wezterm-gui.exe`, `ShowWindow`, `SetForegroundWindow`, `SetWindowPos`,
  `MonitorFromPoint`/`EnumDisplayMonitors` für Monitor 2,
  `SystemParametersInfo(SPI_SETANIMATION)` off/on für Responsiveness.
- Herdr-Bridge: ruft `herdr agent list|focus|start`, `herdr pane list|split`
  (JSON-Parse), toleriert fehlenden Server (Retry).
- CLI: `aid run`, `aid register`, `aid unregister`, `aid status`, `--config`.
  Config: `%APPDATA%\Lars-Win-AI\config.json` (Titel, Monitor, Agent-Name,
  WezTerm-Pfad, Pane-State).
- Keine Konsolenausgabe im Hintergrund (`-ldflags -H=windowsgui`).

### B) WezTerm (`config/wezterm/wezterm.lua`)

- `window_background_opacity = 0.90` (zentraler Testwert),
  `colors.background = "#000000"`, optional `window_decorations = "NONE"`,
  fester Fenstertitel via `format-window-title` = "AI-Assistant".
- `font = wezterm.font 'CaskaydiaCove Nerd Font'`, `font_size`, Ligaturen via
  `harfbuzz_features = { 'calt=1', 'liga=1', 'clig=1' }`.
- `keys`: `Ctrl+Shift+Space` = `ActivateCopyMode`; Vi-artige `copy_mode`-Tabelle.
- Start: `wezterm.exe [--config-file <repo>/config/wezterm/wezterm.lua] start -- herdr`
  → bei Herdr-Exit schließt Fenster.

### C) Herdr (`config/herdr/config.toml`)

- `[ui] window_title = "AI-Assistant"` (stabiles OS-Fenstertitel zum Auffinden).
- Theme (transparentes Panel-BG passend zu WezTerm), `host_cursor` beibehalten.
- Installer führt `herdr integration install opencode` aus (managed Dateien).
- `herdr --skill` → schreibt Skill nach `~/.config/opencode/skills/herdr/SKILL.md`.

### D) Neovim/LazyVim (`config/nvim/*`)

- Bestehende Configs einfalten (`lua/config/{options,keymaps,autocmds,lazy}.lua`,
  `lua/plugins/tokyonight.lua`).
- Markdown-Diagnostics beim Start aus: Autocmd `LspAttach` → wenn
  `filetype=markdown`, `vim.diagnostic.enable(false,{bufnr})`
  (via `<leader>ud` weiter toggelbar).
- `vim.opt.spelllang = { "en", "de" }`.
- `lazy-lock.json` mit übernehmen (reproduzierbare Plugin-Versionen).

### E) opencode

- Theme `config/opencode/themes/ai-transparent.json` mit
  `background/backgroundPanel/backgroundElement/backgroundMenu = "none"`
  (transparent), aktiviert via `"theme"` in `opencode.jsonc`.
- MCP-Merge (idempotent, **kein** Clobber): `exa` (remote) + `playwright`
  (local) ergänzen; zoho/brain bleiben unberührt.
- Skill installieren (`herdr`), `EDITOR`/`VISUAL = nvim` als User-Env setzen.
- Standardaufruf der opencode-Sitzung: **`opencode --auto`**.

### G) Crush

- Herdr erkennt Crush **nativ**: Crush meldet Agent-Zustand und Session-ID selbst
  über den Herdr-Socket (Source `crush`, Agent `crush`); kein
  `herdr integration install crush` nötig.
- Skill (`herdr --skill`) als Junction `config/crush/skill` →
  `%USERPROFILE%/.config/crush/skills/herdr`.

### F) Font

- Eigener `font`-Component: Nerd-Fonts-Release-ZIP (CaskaydiaCove) von GitHub
  laden und die Schnitte **pro Benutzer** registrieren (ohne winget).

## 7. Installer-Ablauf (`install.ps1`)

1. Preflight: winget/git vorhanden, Admin/Developer-Mode-Status melden (nur für
   File-Symlinks nötig), Manifest laden.
2. **Komponentenauswahl** (interaktiv, Default alle; `-Components`/`--yes` für
   non-interaktiv).
3. Deps via winget installieren (überspringt bereits Installiertes).
4. Configs platzieren:
   - Junction (ordnerbasiert, kein Admin): `%LOCALAPPDATA%\nvim` → `config/nvim`
     (bestehendes git-Repo vorher sichern).
   - File-Symlink (Dev-Mode/Admin) für `%APPDATA%\herdr\config.toml` und
     `%USERPROFILE%\.wezterm.lua`; Fallback Copy.
   - Font: Nerd-Fonts-Release laden, Schnitte pro Benutzer registrieren.
   - opencode: Junction nur für `themes/` & `skills/`; `opencode.jsonc` per
     **Merge** patchen (Backup `.bak`).
5. Herdr-Integration + Skill installieren.
6. Daemon: Release laden, Task Scheduler „At log on" registrieren, State-Datei
   anlegen.
7. Verifikation + Journal (`install.log`), idempotent wiederholbar.

## 8. Subagent-Orchestrierung (parallel)

**Orchestrierung über Herdr:** Der bauende Agent orchestriert die parallelen
Subagenten (Phase 1) **über Herdr** (Panes + Agent-Slots), nicht nur lokal.
Der dafür nötige Kontext zur Herdr-Steuerung (`herdr pane …`, `herdr agent …`,
Fokus/Start/Split) wird mit **`herdr --skill`** abgerufen — das ist die
maßgebliche Referenz und **vor** der Orchestrierung zu lesen. Diese Hinweise
gehören auch in `AGENTS.md`, damit künftige Agents sie kennen.

**Phase 0 (sequenziell):** Repo-Scaffold + **eingefrorene Verträge**:
Verzeichnislayout, `manifest.json`-Schema, Installer-Modul-Contract
(`Install-<Component>`), Daemon-Config-Schema + CLI-Flags, Env-Varnamen
(`AID_WINDOW_TITLE`, `AID_MONITOR`, `AID_AGENT_NAME`), Symlink-Zieltabelle.

**Phase 1 (parallel, unabhängig):**

- WS-D: Go-Daemon + Unit-Tests.
- WS-A: WezTerm- + Herdr-Assets.
- WS-N: LazyVim-Assets.
- WS-O: opencode-Assets (Theme, MCP-Merge, Skill).
- WS-I: Installer-Core (lib, deps, Symlink-Logik, Task-Registrierung).
- WS-CI: GitHub-Actions-Release-Workflow + Docs-Gerüst.

**Phase 2 (Integration):** Komponenten verdrahten, README/AGENTS.md final,
lokaler Smoke-Test.

## 9. Risiken / offene Punkte

- **WezTerm-Transparenz unter Windows 11**: GPU-/Treiber-abhängig — muss
  getestet werden; Fallback: voll deckend + nur opencode-Theme transparent.
- **Dead-Key `^` + Alt global**: Low-Level-Hook ist der robuste Weg (ScanCode);
  Kollision mit AltGr links/rechts sauber ausschließen.
- **Symlink-Rechte**: File-Symlinks brauchen Developer Mode/Admin; Junctions
  (Ordner) nicht. Fallback Copy ist eingebaut.
- **opencode-Verzeichnis**: enthält herdr-managed Dateien → nicht komplett
  junctionen.
- **Daemon-Start vs. Herdr-Server**: Socket evtl. noch nicht da → Retry-Logik.
- **`herdr agent start`** braucht ein freies Shell-Pane → Layout-Erstellung nötig.
- **AHK**: `!^`-Eintrag in `Quick Apps.ahk` entfernen, sonst Doppelverarbeitung
  (andere Apps bleiben in AHK).

## 10. Verifikation

- Go: `go vet` + `go test ./...`; CI baut Release-`aid.exe`.
- Installer: Pester-Tests + zweiter Lauf (Idempotenz).
- Manuell: Logon-Autostart; Alt+^ durch alle 4 Zustände; Monitor 2; Font;
  `ctrl+shift+space` Copy; LazyVim markdown ohne Diagnostics + `en,de` Spell;
  opencode transparentes Theme; MCPs vorhanden; Herdr-Skill vorhanden.

## 11. Testing

Details und Befehle: [`docs/TESTING.md`](docs/TESTING.md). Keine VM, keine
Screenshot-Pipeline — alles läuft lokal.

### Stufe 1 — Unit und CI (`windows-latest`), bei jedem Push

- Go: `go vet ./...`, `go test ./...`, Build von `aid.exe` und `bench.exe`.
- PowerShell: PSScriptAnalyzer über `install.ps1`/`scripts/**`; Pester-Unit-Tests
  für `lib.ps1` (Junction/Symlink-Helper, JSONC-Merge, Backup) in Temp-Sandbox.
- Config-Validierung (ohne Systemänderung): TOML-Parse (Herdr),
  JSON-Schema (`manifest.json`, opencode-Theme), `opencode.jsonc`-Merge,
  Lua-Syntax (LazyVim + WezTerm).
- `install.ps1 -DryRun`: asserted, dass nichts geschrieben wird.

### Stufe 2 — Bench (echter Desktop, automatisch)

- `cmd/bench` misst `show`, `hide`, `switch` in-process gegen `internal/apps`
  (keine Tastatur-Injektion), inklusive Flacker-Sampling, Geometrie und Alpha.
- Schwellen: `show` p95 ≤ 50 ms, `hide` ≤ 30 ms, `switch` ≤ 80 ms,
  0 Flacker-Ereignisse. JSON-Report + Baseline-Vergleich, Exitcode 1 bei
  Verletzung.

### Stufe 3 — Subjektive Bewertung (Mensch, lokal)

- `tests/e2e/subjective/run.ps1`: vier Blöcke, Skalen 1–10 für Latenz,
  Smoothness und Gesamterfahrung, plus Flacker ja/nein.
- Ergebnisse als JSON mit Commit, Binary-Hash und Monitorlayout; `-Compare`
  gegen einen früheren Lauf.

### Testmatrix

| # | Szenario | Stufe | Erwartung |
|---|---|---|---|
| T1 | Frischer Komplett-Install | 1 | Exit 0, alle Assertions grün |
| T2 | Zweiter Lauf (Idempotenz) | 1 | Keine Fehler/Duplikate |
| T3 | Teilinstall (`-Components`) | 1 | Nur diese installiert |
| T4 | winget/git fehlt | 1 | Klarer Abbruch, kein Teilzustand |
| T5 | Symlink ohne Dev-Mode | 1 | Copy-Fallback **oder** klarer Hinweis |
| T6 | Logon-Autostart | 2/3 | Daemon läuft |
| T7 | Uninstall | 1 | Sauber, Backups zurück |
| T8 | Kein Herdr-Server beim ersten Alt+^ | 2/3 | Robuster Start, Retry ok |
| T9 | Keine opencode-Session → Alt+^ | 2/3 | Session wird gestartet |
| T10 | Monitor 2 / Transparenz / Latenz | 2/3 | Schwellen + Bewertung ≥ 8 |
| T11 | Bestehende User-Configs | 1 | Merge ohne Clobber, `.bak` da |
| T12 | `opencode.jsonc` mit Kommentaren | 1 | Merge erhält Kommentare |

### Design-Implikationen fürs Repo

- Installer: `-DryRun`/`-WhatIf`, `-Yes`, `-Components`, `-SkipDeps`,
  strukturiertes Ergebnis (JSON), Journal-Log.
- Daemon: `--config`-Override, `herdr`-Pfad-Override (stubbar), optionaler
  `--no-hook`-Modus → ohne echten Hook testbar; „aktiv" wird aus
  `IsForeground` abgeleitet, damit Bench und Daemon sich nicht desynchronisieren.
- Repo: `tests/e2e/subjective/run.ps1`, Pester-Tests,
  `uninstall.ps1 -RestoreBackups`.
