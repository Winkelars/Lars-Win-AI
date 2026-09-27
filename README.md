# Lars-Win-AI

Dream-Workflow für AI-Assistenten unter Windows — kontrolliert installierbar,
reproduzierbar und ohne AutoHotkey-Abhängigkeit.

Ein globaler Hotkey **`Alt + ^`** öffnet, fokussiert oder minimiert ein
randloses WezTerm-Fenster, in dem **Herdr** persistente Panes verwaltet. Der
Hotkey springt dabei gezielt auf das designierte **opencode**-Panel. Als Editor
dient **Neovim (LazyVim)**, die opencode-Sitzung startet standardmäßig mit
`--auto`.

Der vollständige Plan und die Architektur stehen in [`PLAN.md`](PLAN.md), die
eingefrorenen Schnittstellen in [`docs/CONTRACTS.md`](docs/CONTRACTS.md).

## Voraussetzungen

- **Windows 11** (entwickelt/getestet auf 21H2+ und 26100).
- **winget** (App Installer) für die Abhängigkeiten — `winget --version` muss
  funktionieren.
- **git** (für den Clone des Repos).
- **Entwicklermodus oder Administratorrechte** *nur* für echte Datei-Symlinks.
  Das Repo bevorzugt Junctions (Ordner), die **kein** Admin benötigen; der
  File-Symlinks (`herdr/config.toml`, `wezterm.lua`) fallen automatisch auf
  Kopieren zurück, wenn Developer Mode/Admin fehlt. Hinweis: Developer Mode ist unter
  *Einstellungen → System → Entwickler* aktivierbar.
- Optional: **PowerShell 7** (`pwsh`) — die Skripte laufen auch mit Windows
  PowerShell 5.1.

## Installation

```powershell
git clone https://github.com/Winkelars/Lars-Win-AI.git
Set-Location Lars-Win-AI
pwsh -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```

Der Installer ist interaktiv, idempotent und schreibt ein Journal
(`install.log`). Er ist über folgende Parameter steuerbar:

| Parameter | Bedeutung |
|---|---|
| `-Components <string[]>` | Nur die genannten Komponenten installieren (IDs siehe Tabelle). |
| `-Yes` | Non-interaktiv; alle `default`-Komponenten ohne Rückfrage. |
| `-DryRun` | Nur planen — es wird **nichts** geschrieben (kein Journal, keine Backups). |
| `-SkipDeps` | winget-Schritt überspringen (z. B. für bereits gepflegte Systeme). |
| `-Config <path>` | Pfad zu einer abweichenden Daemon-Config (Default `%APPDATA%\Lars-Win-AI\config.json`). |

Beispiele:

```powershell
# Unbeaufsichtigter Komplett-Install
pwsh -NoProfile -File .\install.ps1 -Yes

# Nur WezTerm + Herdr-Configs, ohne winget
pwsh -NoProfile -File .\install.ps1 -Yes -SkipDeps -Components wezterm,herdr

# Trockenlauf (ändert nichts)
pwsh -NoProfile -File .\install.ps1 -DryRun -Yes
```

Exitcodes: `0` Erfolg, `1` Fehler, `2` Preflight fehlgeschlagen
(z. B. winget/git fehlen).

## Deinstallation

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\uninstall.ps1 -RestoreBackups
```

`-RestoreBackups` stellt die vorher gesicherten Original-Konfigurationen
(`*.bak.<ts>`) wieder her und entfernt Junctions/Symlinks sowie den
Scheduled Task. Ohne den Schalter bleiben Backups liegen.

## Komponenten

Quelle der Wahrheit ist [`manifest.json`](manifest.json); Reihenfolge und
Abhängigkeiten sind dort fixiert.

| ID | Funktion | Reihenfolge | Voraussetzung | Default | Beschreibung |
|---|---|---|---|---|---|
| `deps` | `Install-Deps` | 10 | – | ja | winget: Herdr, WezTerm, Neovim. |
| `font` | `Install-Font` | 15 | – | ja | CaskaydiaCove Nerd Font aus dem Nerd-Fonts-Release (pro Benutzer, ohne winget). |
| `wezterm` | `Install-Wezterm` | 20 | `deps` | ja | WezTerm-Config (`~/.wezterm.lua`) platzieren. |
| `herdr` | `Install-Herdr` | 30 | `deps` | ja | Herdr-Config verlinken, opencode-Integration + Skill installieren. |
| `neovim` | `Install-Neovim` | 40 | `deps` | ja | LazyVim-Config als Junction (`%LOCALAPPDATA%\nvim`), bestehendes Repo vorher sichern. |
| `opencode` | `Install-Opencode` | 50 | `deps` | ja | Theme/Skill verlinken, MCP + Theme in `opencode.jsonc` mergen, `EDITOR`/`VISUAL` setzen. |
| `daemon` | `Install-Daemon` | 60 | – | ja | `aid.exe` aus dem GitHub-Release laden, Config/State anlegen, Scheduled Task registrieren. |

`opencode.jsonc` wird **nicht** gejunctiont, sondern per Merge gepatcht
(Kommentare bleiben erhalten, vorhandene Keys wie `zoho`/`brain` werden **nicht**
überschrieben, vorher entsteht ein `.bak`).

## Hotkey und Zustandsmaschine

Der Go-Daemon `aid.exe` registriert einen globalen Low-Level-Hook auf
`Alt + ^` (layoutunabhängig über ScanCode `0x29`; AltGr wird ausgeschlossen) und
startet über den Task Scheduler „At log on“ automatisch.

`Alt + ^` verhält sich je nach Zustand:

1. **Fenster existiert nicht** → WezTerm + `herdr` starten, auf Monitor 2
   maximieren/fokussieren, opencode-Pane fokussieren (ggf. starten).
2. **Fenster existiert, nicht im Vordergrund** → nach vorn holen (ggf. auf
   Monitor 2 ziehen), opencode-Pane fokussieren.
3. **Fenster im Vordergrund, opencode-Pane nicht fokussiert** → nur das
   opencode-Pane fokussieren (schnell).
4. **Fenster im Vordergrund und opencode-Pane fokussiert** → minimieren.

Existiert noch kein opencode-Agent, wird der designierte Pane aus der
State-Datei genutzt bzw. per `herdr pane split` erzeugt und
`herdr agent start … -- --auto` gestartet.

## Troubleshooting

### Fenster ist nicht transparent (GPU/Treiber)

WezTerm-Transparenz unter Windows 11 hängt von GPU und Treiber ab. Zeigt das
Fenster keine Transparenz, ist der Fallback vollständig deckend — nur das
opencode-Theme bleibt transparent. Prüfen:

```powershell
wezterm --version
# WezTerm-Treiber/GPU aktualisieren
```

Der zentrale Transparenzwert liegt in `config/wezterm/wezterm.lua`
(`window_background_opacity`).

### Symlink-/Junction-Fehler

- Ordner werden als **Junctions** verlinkt und brauchen kein Admin.
- Nur `%APPDATA%\herdr\config.toml` und `%USERPROFILE%\.wezterm.lua` sind **File-Symlinks**; sie benötigen
  Developer Mode oder Admin. Fehlt beides, fällt der Installer auf **Kopieren**
  zurück.
- Bestehende Ziele werden vorher gesichert (`*.bak.<ts>`).
- Developer Mode aktivieren: *Einstellungen → System → Entwickler →
  Entwicklermodus*.

### Herdr-Socket nicht erreichbar

Der Daemon toleriert einen noch nicht laufenden Herdr-Server und wiederholt den
Zugriff (`herdr_retry_count × herdr_retry_ms`). Prüfen:

```powershell
herdr status
Test-Path "$env:APPDATA\herdr\herdr.sock"
```

Läuft der Server nicht, Herdr einmal interaktiv starten; danach `Alt + ^` erneut
drücken.

### opencode-Session startet nicht

- `herdr agent list` prüfen: existiert ein opencode-Agent?
- Ohne freies Shell-Pane muss der Daemon erst `herdr pane split` ausführen.
- Auto-Approve: Die Session wird mit `--auto` gestartet (siehe `env` in
  `manifest.json`).

## Entwicklung und Tests

- CI (GitHub Actions, `windows-latest`): Go-Vet/-Test/-Build,
  PSScriptAnalyzer, Pester und die Config-Validierung
  `tests/ci/validate-configs.ps1`.
- Release: Ein Tag `v*` baut `aid.exe` (`-H=windowsgui -s -w`) und hängt ihn an
  das GitHub-Release.
- Teststrategie inkl. Layer 1–3 und Testmatrix T1–T12:
  [`docs/TESTING.md`](docs/TESTING.md).

```powershell
go vet ./...
go test ./...
go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid
Invoke-Pester -Path tests/pester
Invoke-ScriptAnalyzer -Path . -Recurse
pwsh -NoProfile -File tests/ci/validate-configs.ps1
```

## Verweise

- [`PLAN.md`](PLAN.md) — Zielbild, Architektur, Phasen.
- [`docs/CONTRACTS.md`](docs/CONTRACTS.md) — eingefrorene Verträge (Pfade,
  Funktionen, Schemata, CLI).
- [`docs/TESTING.md`](docs/TESTING.md) — Test-Layer und Matrix.
