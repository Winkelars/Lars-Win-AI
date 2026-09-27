# Lars-Win-AI — Teststrategie

Dieses Dokument fasst die Test-Layer und die Testmatrix aus
[`PLAN.md`](../PLAN.md) §11 zusammen und ergänzt die konkret ausführbaren
Befehle. Es ist die Grundlage für die CI in
[`.github/workflows/ci.yml`](../.github/workflows/ci.yml).

**Annahme:** Die Test-VM (Windows 11 21H2 Dev-Env) und das Zielsystem (26100)
verhalten sich für unsere Prüfpunkte identisch. Es wird **kein** separater
Parity-Test mit einer plainen Windows-11-ISO gefahren.

## Layer 1 — CI (GitHub Actions, `windows-latest`)

Läuft bei jedem Push und Pull Request. Alle Schritte sind so gebaut, dass noch
nicht existierende Workstream-Dateien den Job **nicht** rot machen
(Existenz-Check → `::notice::`), vorhandene Dateien aber **zwingend** validiert
werden.

| Schritt | Kommando / Werkzeug | Prüft |
|---|---|---|
| Go Vet | `go vet ./...` | Statische Fehler in `cmd/**`, `internal/**`. |
| Go Test | `go test ./...` | Unit-Tests (`internal/toggle`, `config`, `herdr`, `window`). |
| Go Build | `go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid` | Windows-Binary ohne Konsole. |
| PSScriptAnalyzer | `Invoke-ScriptAnalyzer` über `install.ps1`, `uninstall.ps1`, `scripts/**`, `tests/**` | PowerShell-Qualität. Fehler = rot, Warnungen = Annotation. |
| Pester | `Invoke-Pester -Path tests/pester` | Installer-Helfer (Junction/Symlink, JSONC-Merge, Backup). |
| Config-Validierung | `tests/ci/validate-configs.ps1` | JSON (`manifest.json`, Theme), TOML (Alacritty, Herdr), Lua-Syntax (LazyVim), `install.ps1 -DryRun`. |

Details der Config-Validierung:

- **JSON:** `manifest.json` wird strukturell geprüft (Komponenten-IDs,
  `function == Install-<PascalId>`, eindeutige `order`, Symlink-Felder,
  `release.asset == aid.exe`). opencode-Themes werden geparst; beim Theme
  `ai-transparent.json` müssen `background`, `backgroundPanel`,
  `backgroundElement` und `backgroundMenu` den Wert `"none"` haben.
- **TOML:** `config/alacritty/alacritty.toml` und `config/herdr/config.toml`
  werden mit dem Node-Paket `toml` geparst (siehe
  `tests/ci/validate-toml.js`).
- **Lua:** Jede `*.lua` unter `config/nvim/**` wird per
  `nvim --headless -u NONE -c "lua … loadfile(...)"` auf Syntax geprüft.
- **DryRun:** Existiert `install.ps1`, läuft `install.ps1 -DryRun -Yes` und darf
  **kein** `install.log` schreiben.

### Lokale Befehle (Layer 1)

```powershell
# Go
go vet ./...
go test ./...
go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid

# PowerShell
Invoke-Pester -Path tests/pester
Invoke-ScriptAnalyzer -Path . -Recurse

# Config-Validierung (identisch zur CI)
pwsh -NoProfile -File tests/ci/validate-configs.ps1
```

## Layer 2 — Windows-VM (Hyper-V), E2E

- Basis: Hyper-V Quick Create „Windows 11-Entwicklungsumgebung“, 2 vCPU / 4 GB,
  lokaler Admin `User` / `Passw0rd!`, **Checkpoint `baseline`** nach dem
  Erstboot.
- Vor jedem Lauf: Rollback auf `baseline`.
- `tests/e2e/run.ps1`: `install.ps1 -Yes` → Assertions (Pakete, Junctions,
  Scheduled Task, Daemon-Prozess, `EDITOR`/`VISUAL`, opencode-Merge **ohne**
  Clobber von `zoho`/`brain`, Herdr-Skill-Datei, Font, Config-Parse) →
  **Reboot** → At-Logon → **zweiter Lauf** (Idempotenz) →
  `uninstall.ps1 -RestoreBackups` → Aufräum-Assertions.
- Interaktiv in der VM: `Alt + ^` durch alle 4 Zustände, opencode-Pane
  focus/start, `ctrl+shift+space`, LazyVim-Markdown ohne Diagnostics + `en,de`
  Spell, transparentes Theme.

## Layer 3 — Abnahme auf echter Hardware (Pflicht)

- Dediziertes lokales Testkonto auf dem 26100-Zielsystem: Monitor 2, echte
  Transparenz, Responsiveness; danach Lauf im Hauptprofil.
- Stellt Layer 3 eine Abweichung gegenüber der 21H2-VM fest, wird Layer 2
  nachgezogen.

## Testmatrix

| # | Szenario | Layer | Erwartung |
|---|---|---|---|
| T1 | Frischer Komplett-Install | VM | Exit 0, alle Assertions grün |
| T2 | Zweiter Lauf (Idempotenz) | VM | Keine Fehler/Duplikate |
| T3 | Teilinstall (`-Components`) | VM | Nur diese installiert |
| T4 | winget/git fehlt | VM | Klarer Abbruch, kein Teilzustand |
| T5 | Symlink ohne Dev-Mode | VM | Copy-Fallback **oder** klarer Hinweis |
| T6 | Reboot → Autostart | VM | Daemon läuft |
| T7 | Uninstall | VM | Sauber, Backups zurück |
| T8 | Kein Herdr-Server beim ersten `Alt + ^` | VM | Robuster Start, Retry ok |
| T9 | Keine opencode-Session → `Alt + ^` | VM | Session wird gestartet |
| T10 | Monitor 2 / Transparenz / Responsiveness | Real | Akzeptanz |
| T11 | Bestehende User-Configs | VM/Real | Merge ohne Clobber, `.bak` da |
| T12 | `opencode.jsonc` mit Kommentaren | VM | Merge erhält Kommentare |

## Design-Implikationen fürs Repo

- Installer: `-DryRun`/`-WhatIf`, `-Yes`, `-Components`, `-SkipDeps`, ein
  strukturiertes Ergebnis (JSON) und Journal-Log.
- Daemon: `--config`-Override, `herdr`-Pfad-Override (stubbar) und optionaler
  `--no-hook`-Modus → in VM/CI ohne echten Hook testbar.
- Repo: `tests/e2e/{run.ps1,MANUAL.md}`, Pester-Tests,
  `uninstall.ps1 -RestoreBackups`.
