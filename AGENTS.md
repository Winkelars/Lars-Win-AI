# AGENTS.md — Konventionen für Agents in Lars-Win-AI

Diese Datei richtet sich an **künftige Agents** (opencode, Claude, Codex, …),
die an diesem Repo arbeiten. Sie ergänzt [`PLAN.md`](PLAN.md) und
[`docs/CONTRACTS.md`](docs/CONTRACTS.md).

## Sprache & Stil

- **Antworten und Dokumentation auf Deutsch.** Technische Bezeichner,
  Dateinamen, CLI-Flags, Code und Commit-Typen bleiben Englisch.
- Commit-Messages: knappe „low-effort summaries" auf Englisch (Repo-Stil).
- Keine Emojis in Dateien, außer explizit gewünscht.

## Umgebung

- **Windows 11**, Shell **PowerShell 5.1** (kein `&&`; `cmd1; if ($?) { cmd2 }`).
- Paketmanager: **winget** (dann scoop/uv).
- Repos liegen unter `C:\Users\Makkis\Documents\codestuff\<projekt>`.
- PowerShelI-/Datei-Encoding: **UTF-8**.

## Repo-Regeln

- **Nicht committen**, außer der Nutzer/Orchestrator fordert es ausdrücklich.
- Keine Secrets, keine Binaries (`.exe`) einchecken — siehe `.gitignore`.
- **Nur Go-Standardbibliothek** im Daemon (kein `golang.org/x/sys` o. Ä.),
  damit offline/CI-Builds zuverlässig sind.
- Verträge in `docs/CONTRACTS.md` sind **eingefroren**: Dateibesitz, Funktionen,
  Schemata, Env-Varnamen exakt einhalten.
- Vor Abgabe: `go vet ./...`, `go test ./...`, PSScriptAnalyzer, Pester,
  Config-Parse (siehe `docs/TESTING.md`).

## Herdr-Orchestrierung

Referenz ist **`herdr --skill`** (maßgeblich). Kurzfassung für die Steuerung:

```powershell
# Pflicht: in einer Herdr-managed Pane
if ($env:HERDR_ENV -ne '1') { throw 'Nicht in Herdr' }

# Zustand
herdr status
herdr agent list                     # JSON: .result.agents[] (agent, focused, pane_id, agent_status)
herdr pane list
herdr pane layout --pane $env:HERDR_PANE_ID

# Layout erzeugen (neuer Tab / Splits)
herdr tab create --cwd <path> --label work --no-focus   # -> .result.tab, .result.root_pane
herdr pane split <pane_id> --direction right --no-focus  # -> .result.pane.pane_id

# Agent starten (nur in freier Shell-Pane) und koordinieren
herdr agent start ws-d --kind opencode --pane <pane_id> -- --auto
herdr agent prompt ws-d "<Auftrag>" --wait --timeout 600000
herdr agent read   ws-d --source recent-unwrapped --lines 120
herdr agent get    ws-d
```

Regeln:
- **Orchestrator-Tab bleibt exklusiv**; Subagenten laufen in einem **neuen Tab**
  in **Split-Panes** (`--no-focus`).
- IDs **immer** aus JSON (`.result.*`) lesen — nie raten/aus Reihenfolge ableiten.
- `herdr agent start` nie auf einer Pane mit laufendem Editor/Command; vorher
  `herdr pane list` prüfen.
- Zielsetzung: Subagenten dürfen nicht committen; Ergebnisse bleiben im
  Working Tree.

## Build & Test

```powershell
# Go
go vet ./...
go test ./...
go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid

# PowerShell
Invoke-Pester tests/pester
Invoke-ScriptAnalyzer -Path . -Recurse
```

## Verweise

- Plan: [`PLAN.md`](PLAN.md)
- Verträge: [`docs/CONTRACTS.md`](docs/CONTRACTS.md)
- Teststrategie: [`docs/TESTING.md`](docs/TESTING.md)
