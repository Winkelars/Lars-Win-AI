# Lars-Win-AI — Dream-Workflow für AI-Assistenten unter Windows

Ziel: Ein GitHub-Repo, das den kompletten Windows-Workflow um einen
AI-Assistenten **kontrolliert** installiert und reproduzierbar macht. Ersetzt
AutoHotkey als Abhängigkeit durch einen eigenen Hintergrundprozess.

## 1. Zielsetup

- **Hotkey `Alt + ^` (Zirkumflex)** öffnet / minimiert / fokussiert das
  AI-Assistenten-Fenster je nach Ursprungszustand (wie das bisherige
  AHK-Skript, siehe Referenz `Quick Apps.ahk`).
- Das Fenster ist effektiv **eine Alacritty-Instanz**; darin läuft **Herdr**.
- Beim Beenden von Herdr schließt sich das Alacritty-Fenster.
- Herdr dient auch für andere PowerShell-Sitzungen. Der Hotkey soll aber gezielt
  das **designierte opencode-Panel** fokussieren.
- Arbeiter der User bereits in Herdr, soll der Hotkey **schnell direkt zum
  opencode-Panel wechseln**.
- **Neovim (LazyVim)** als Editor in opencode:
  - Markdown-Dateien: Diagnostics per Default **aus** (`<leader>ud` zum Togglen).
  - Spellchecking **Deutsch + Englisch**.
- **Alacritty**:
  - Font **„CaskaydiaCove Nerd Font"** (über Repo automatisch bereitstellbar).
  - `ctrl+shift+space` (Visual Mode / Copy) = Standardwert, essenziell.
  - Windows-native Titelleiste **ausgeblendet** (nackt).
  - Leicht transparent (Wert offen, zentral konfigurierbar).
- **opencode**: Theme mit **100 % transparentem Hintergrund**, falls möglich.
- **`herdr --skill`** wird als opencode-Skill installiert.
- **Playwright-MCP** und **Exa Search MCP** werden konfiguriert.

## 2. Ist-Zustand (verifiziert auf dem Referenzsystem)

| Komponente | Status | Detail |
|---|---|---|
| AHK-Skript | aktiv | `Startup\Quick Apps.ahk`; `!^` toggelt heute das **DeepSeek**-Chromium-Fenster (nicht Alacritty) |
| Alacritty | installiert (winget), **keine Config** | `C:\Program Files\Alacritty\alacritty.exe`, 0.17.0, Config-Default `%APPDATA%\alacritty\alacritty.toml` |
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
        Alacritty (borderless, CaskaydiaCove, opacity,
        ctrl+shift+space vi-mode)  -e herdr
                │
        Herdr-Server (persistente Panes)
                ├─ Pane: opencode  ← designiertes Pane
                └─ Pane: PowerShell-Sessions
                        opencode → Editor nvim, Theme transparent,
                                   MCP exa+playwright, Skill herdr
```

### Toggle-Zustandsmaschine (Alt+^)

1. Fenster existiert nicht → Alacritty + `herdr` starten → Monitor 2
   maximieren/fokussieren → opencode-Pane fokussieren (ggf. starten).
2. Fenster existiert, nicht Vordergrund → nach vorn holen (restore), ggf. auf
   Monitor 2 ziehen → opencode-Pane fokussieren.
3. Fenster Vordergrund, opencode-Pane **nicht** fokussiert → nur opencode-Pane
   fokussieren (schnell).
4. Fenster Vordergrund **und** opencode-Pane fokussiert → minimieren.

### opencode-Pane-Auflösung

`herdr agent list` → Agent `kind=opencode` vorhanden → `herdr agent focus <name>`.
Nicht vorhanden → designierten Pane (persistiert in State-Datei, validiert)
nutzen, sonst `herdr pane split` + `herdr agent start <name> --kind opencode
--pane <id>`.

## 4. Entschiedene Design-Fragen

| Frage | Entscheidung |
|---|---|
| Daemon-Sprache | **Go** (eine statische `.exe`, keine Runtime) |
| Config-Verwaltung | **Symlink/Junction** (Repo = Single Source of Truth) |
| Daemon-Auslieferung | **GitHub-Actions-Release**, Installer lädt `aid.exe` |
| Herdr-Fokus + opencode-Pane nicht fokussiert | **Zum opencode-Panel springen** (nicht minimieren) |
| Keine opencode-Session | **opencode-Session starten** |
| Autostart | **Task Scheduler** (At log on) |

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
│     ├─ deps.ps1   (winget: herdr, alacritty, neovim, CaskaydiaCove)
│     ├─ alacritty.ps1
│     ├─ herdr.ps1
│     ├─ neovim.ps1
│     ├─ opencode.ps1
│     └─ daemon.ps1              # Release laden, Task registrieren
├─ config/
│  ├─ alacritty/alacritty.toml
│  ├─ herdr/config.toml
│  ├─ nvim/{init.lua,lua/config/*,lua/plugins/*,lazyvim.json,lazy-lock.json}
│  └─ opencode/{theme json, mcp snippet, skill}
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
  `alacritty.exe`, `ShowWindow`, `SetForegroundWindow`, `SetWindowPos`,
  `MonitorFromPoint`/`EnumDisplayMonitors` für Monitor 2,
  `SystemParametersInfo(SPI_SETANIMATION)` off/on für Responsiveness.
- Herdr-Bridge: ruft `herdr agent list|focus|start`, `herdr pane list|split`
  (JSON-Parse), toleriert fehlenden Server (Retry).
- CLI: `aid run`, `aid register`, `aid unregister`, `aid status`, `--config`.
  Config: `%APPDATA%\Lars-Win-AI\config.json` (Titel, Monitor, Agent-Name,
  Alacritty-Pfad, Pane-State).
- Keine Konsolenausgabe im Hintergrund (`-ldflags -H=windowsgui`).

### B) Alacritty (`config/alacritty/alacritty.toml`)

- `[window] decorations = "None"`, `opacity = 0.90` (zentraler Testwert),
  `dynamic_title = true`, `title = "AI-Assistant"`, `padding`, `startup_mode`.
- `[font] family = "CaskaydiaCove Nerd Font"`, `size`.
- `[keyboard]` `ToggleViMode` = `Ctrl+Shift+Space` (Default, explizit
  festschreiben).
- Start: `alacritty.exe -T "AI-Assistant" --config-file <repo>/config/alacritty/alacritty.toml -e herdr`
  → bei Herdr-Exit schließt Fenster (Default).

### C) Herdr (`config/herdr/config.toml`)

- `[ui] window_title = "AI-Assistant"` (stabiles OS-Fenstertitel zum Auffinden).
- Theme (transparentes Panel-BG passend zu Alacritty), `host_cursor` beibehalten.
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

### F) Font

- `winget install ryanoasis.CaskaydiaCove` (idempotent; bereits vorhanden).

## 7. Installer-Ablauf (`install.ps1`)

1. Preflight: winget/git vorhanden, Admin/Developer-Mode-Status melden (nur für
   File-Symlinks nötig), Manifest laden.
2. **Komponentenauswahl** (interaktiv, Default alle; `-Components`/`--yes` für
   non-interaktiv).
3. Deps via winget installieren (überspringt bereits Installiertes).
4. Configs platzieren:
   - Junction (ordnerbasiert, kein Admin): `%APPDATA%\alacritty` →
     `config/alacritty`, `%LOCALAPPDATA%\nvim` → `config/nvim` (bestehendes
     git-Repo vorher sichern).
   - File-Symlink (Dev-Mode/Admin) für `%APPDATA%\herdr\config.toml`; Fallback Copy.
   - opencode: Junction nur für `themes/` & `skills/`; `opencode.jsonc` per
     **Merge** patchen (Backup `.bak`).
5. Herdr-Integration + Skill installieren.
6. Daemon: Release laden, Task Scheduler „At log on" registrieren, State-Datei
   anlegen.
7. Verifikation + Journal (`install.log`), idempotent wiederholbar.

## 8. Subagent-Orchestrierung (parallel)

**Phase 0 (sequenziell):** Repo-Scaffold + **eingefrorene Verträge**:
Verzeichnislayout, `manifest.json`-Schema, Installer-Modul-Contract
(`Install-<Component>`), Daemon-Config-Schema + CLI-Flags, Env-Varnamen
(`AID_WINDOW_TITLE`, `AID_MONITOR`, `AID_AGENT_NAME`), Symlink-Zieltabelle.

**Phase 1 (parallel, unabhängig):**

- WS-D: Go-Daemon + Unit-Tests.
- WS-A: Alacritty- + Herdr-Assets.
- WS-N: LazyVim-Assets.
- WS-O: opencode-Assets (Theme, MCP-Merge, Skill).
- WS-I: Installer-Core (lib, deps, Symlink-Logik, Task-Registrierung).
- WS-CI: GitHub-Actions-Release-Workflow + Docs-Gerüst.

**Phase 2 (Integration):** Komponenten verdrahten, README/AGENTS.md final,
lokaler Smoke-Test.

## 9. Risiken / offene Punkte

- **Alacritty-Transparenz unter Windows 11**: bekannter GPU-/Treiber-Fall
  (`supports_transparency: false`) — muss getestet werden; Fallback: voll
  deckend + nur opencode-Theme transparent.
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
