# Lars-Win-AI — Teststrategie

Drei Stufen, alle lokal. Es gibt **keine VM** und keine geräteunabhängige
Screenshot-Pipeline: alles, was die Slot-Umschaltung betrifft, wird auf dem
echten Desktop gemessen.

## Stufe 1 — Unit und CI (kein Desktop nötig)

Läuft bei jedem Push und Pull Request und lokal vor jeder Abgabe.

| Schritt | Kommando | Prüft |
|---|---|---|
| Go Vet | `go vet ./...` | Statische Fehler in `cmd/**`, `internal/**`. |
| Go Test | `go test ./...` | `apps`, `config`, `herdr`, `hotkey`, `toggle`, `window`. |
| Go Build | `go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid` | Daemon-Binary ohne Konsole. |
| Bench Build | `go build -o bin/bench.exe ./cmd/bench` | Bench-Binary. |
| PSScriptAnalyzer | `Invoke-ScriptAnalyzer` über `install.ps1`, `uninstall.ps1`, `scripts/**`, `tests/**` | PowerShell-Qualität. |
| Pester | `Invoke-Pester -Path tests/pester` | Installer-Helfer. |
| Config-Validierung | `tests/ci/validate-configs.ps1` | JSON/TOML/Lua, `install.ps1 -DryRun`. |

```powershell
go vet ./...
go test ./...
go build -ldflags "-H=windowsgui" -o bin/aid.exe ./cmd/aid
go build -o bin/bench.exe ./cmd/bench
Invoke-Pester -Path tests/pester
Invoke-ScriptAnalyzer -Path . -Recurse
pwsh -NoProfile -File tests/ci/validate-configs.ps1
```

## Stufe 2 — Automatischer Bench (echter Desktop)

`cmd/bench` misst die Slot-Logik in-process, also denselben Code, den der
Daemon ausführt. Es wird **keine** Tastatur injiziert: `hotkey.Matches`
verwirft injizierte Events bewusst, und der Hook selbst kostet unter 1 ms. Der
Bench braucht einen angemeldeten Desktop, laeuft aber ohne Admin.

```powershell
go build -o bin/bench.exe ./cmd/bench
bin\bench.exe --iterations 20 --a vivaldi --b terminal
```

Optionen:

| Flag | Bedeutung |
|---|---|
| `--a`, `--b` | Slots (Default: erste zwei `apps` aus der Config) |
| `--iterations` | Messwiederholungen je Metrik (Default 20) |
| `--timeout` | Zeitlimit je Messung in ms (Default 2000) |
| `--max-show`, `--max-hide`, `--max-switch` | p95-Grenzen in ms |
| `--baseline` | frueherer Report als Vergleichsbasis |
| `--out` | Zielverzeichnis (Default `%LOCALAPPDATA%\Lars-Win-AI\bench`) |
| `--json` | Report nach stdout statt Tabelle |
| `--dry-run` | nur Config und Slots pruefen |
| `--exclusive` | beim Zeigen auch alle anderen Slots verstecken (Daemon-Verhalten) |
| `--force` | Schutzsperren umgehen (nur bewusst) |

**Schutz vor dem eigenen Fenster:** Der Bench laeuft oft in einem Terminal, das
selbst ein Slot ist. Deshalb:

- Slots mit Terminal-Prozessen (`WindowsTerminal`, `wezterm-gui`, `conhost`,
  `pwsh`, ...) werden nur mit `--force` gemessen; die Default-Slots ueberspringen
  sie.
- Ist der Vordergrund ein Terminal, bricht der Lauf ab (sonst wuerde er dem
  Benutzer den Bildschirm wegraeumen). `--force` hebt das auf.
- Standardmaessig wird **nur der gemessene Slot** gezeigt und versteckt, nicht
  die Welt drumherum. `--exclusive` schaltet das Daemon-Verhalten dazu.
- Vor dem Lauf laufen 2 Sekunden Countdown; der Ausgangszustand wird per
  `defer` wiederhergestellt, auch wenn eine Messung mit Fehler abbricht.

**Szenarien:**

| Metrik | Ablauf | Default-Grenze |
|---|---|---|
| `show` | versteckter Slot wird geholt | p95 ≤ 50 ms |
| `hide` | sichtbarer Slot wird versteckt | p95 ≤ 30 ms |
| `switch` | A ↔ B im Wechsel, inkl. Flacker-Sampling | p95 ≤ 80 ms |
| `<slot> cold` | App wird gestartet (nur wenn nicht laufend) | informativ |

Zusätzlich:

- **Flacker-Sampling.** Waehrend jedes Wechsels wird alle ~1 ms gezaehlt, wie
  viele Slot-Fenster sichtbar sind. Ein Wert von 0 bedeutet: der Desktop hat
  durchgeblitzt. Grenze: 0 Ereignisse.
- **Geometrie.** Nach dem Zeigen muss das Fensterrechteck der Work Area des
  konfigurierten Monitors entsprechen (±2 px).
- **Alpha.** Nur Slots mit `alpha` zwischen 1 und 254 muessen den Wert tragen;
  Terminal, WezTerm und Pear Desktop bleiben nativ (und werden nicht
  angefasst).

Der Exitcode ist 1, sobald eine Grenze verletzt ist. Reports landen als JSON in
`--out`; `--baseline` zeigt die Delta-Werte (p95) zum letzten Lauf.

> Der Bench blendet echte Fenster ein und aus. Nicht waehrend laufender Arbeit
> starten — die Schutzsperren oben fangen den Normalfall ab.

## Stufe 3 — Subjektive Bewertung (Mensch, lokal)

Automatische Zahlen sagen nichts ueber Wahrnehmung. `tests/e2e/subjective/run.ps1`
fuehrt durch vier Bloecke und fragt danach drei Skalen ab.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\e2e\subjective\run.ps1
# Vergleich mit einem frueheren Lauf:
... -Compare "$env:LOCALAPPDATA\Lars-Win-AI\bench\subjective\subjective-20261118-201500.json"
```

**Bloecke:** Umschalten derselben App, Wechsel zwischen zwei Apps, schneller
Wechsel, Kaltstart. Davor ein Einlauf-Block, der nicht gewertet wird.

**Skalen (1–10), Anker werden bei jeder Abfrage eingeblendet:**

| Skala | 1 | 5 | 10 |
|---|---|---|---|
| Latenz | traege | merkbar | sofort |
| Smoothness | ruckelt/flackert | geht so | butterweich |
| Gesamterfahrung | unbrauchbar | brauchbar | perfekt |

Dazu `Flackern gesehen?` (ja/nein) und ein optionaler Kommentar.

Bewertet wird **vor** der Anzeige irgendwelcher Messwerte, damit die Zahlen die
Wahrnehmung nicht ankern. Die Blockreihenfolge bleibt fest, damit Laeufe ueber
Wochen vergleichbar sind; den Reihenfolge-Bias faengt der Einlauf ab.

Ergebnisse liegen als JSON **ausserhalb des Repos**
(`%LOCALAPPDATA%\Lars-Win-AI\bench\subjective\`) und enthalten Commit,
`aid.exe`-Hash mit Build-Zeit, Config-Hash und Monitorlayout. Ohne diese
Metadaten sind zwei Bewertungen nicht vergleichbar.

## Interpretation

Referenz auf dem Zielsystem (Terminal ↔ Explorer, 10 Iterationen):
`show` p50 5,1 ms / p95 7,4 ms, `hide` p50 3,0 ms, `switch` p50 21,0 ms,
0 Flacker-Ereignisse. Das alte AHK-Skript lag durch die 100-ms-Autodelays und
das Minimieren aller anderen Fenster bei 600–1000 ms pro Wechsel.

- `show` ueber 50 ms p95: meist ein falsch gesetztes Alpha, ein Dialog im Slot
  oder eine App, die `SW_SHOW` verschleppt (Steam-Kandidat).
- `switch` deutlich langsamer als `show`: das Verstecken der alten Fenster
  blockiert; dann pruefen, ob ein Slot versehentlich alle `chrome.exe`-Fenster
  matcht.
- Flacker-Ereignisse > 0: die Reihenfolge in `apps.show` ist verletzt (zeigen
  vor verstecken) oder ein Slot wird doppelt gematcht.
- Subjektiv schlecht bei guten Zahlen: Sichtbarkeit/Transparenz pruefen, nicht
  die Latenz.

## Design-Implikationen

- Der Daemon braucht `--config`, `--no-hook` und einen resettbaren Zustand
  („aktiv" wird aus `IsForeground` abgeleitet, nicht gecacht), damit Bench und
  Daemon sich nicht desynchronisieren.
- `internal/apps` bleibt frei von Plattform-Code und damit auf jeder Plattform
  testbar; die echten Syscalls liegen in `internal/platform`.
- Neue Slots brauchen keinen Testcode: sie laufen durch dieselben Szenarien.
