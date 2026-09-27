# Layer-2 / Layer-3 E2E — manuelle Anleitung

Ergänzt `tests/e2e/run.ps1` (Layer 2, automatisierter Teil) um die manuellen
Schritte aus `PLAN.md` §11. Zielsystem: Hyper-V-Wegwerf-VM (Win 11 Dev-Env),
Baseline-Checkpoint `baseline`.

## Voraussetzungen

- Hyper-V Quick Create „Windows 11-Entwicklungsumgebung", 2 vCPU / 4 GB,
  lokaler Admin `User` / `Passw0rd!`.
- Checkpoint `baseline` nach Erstboot anlegen.
- **Vor jedem Lauf** Rollback auf `baseline`.

## Layer 2 — automatisierter Durchlauf

```powershell
# In der VM, Repo unter z. B. C:\Lars-Win-AI
powershell -NoProfile -ExecutionPolicy Bypass -File tests\e2e\run.ps1
```

Das Skript führt aus:

1. `install.ps1 -Yes` und prüft:
   - Junction/Symlink-Ziele laut `manifest.json`,
   - Scheduled Task `Lars-Win-AI\aid`,
   - `EDITOR`/`VISUAL = nvim`,
   - `opencode.jsonc`-Merge (exa/playwright vorhanden, zoho/brain **nicht**
     überschrieben, Theme `ai-transparent`, JSONC parst),
   - Herdr-Skill-Datei, Font, `aid.exe`, Daemon-`config.json`.
2. **Reboot-Hinweis** → VM neu starten, danach erneut
   `install.ps1 -Yes` (Idempotenz: keine Fehler/Duplikate).
3. `uninstall.ps1 -RestoreBackups` und prüft, dass Links/Task entfernt und
   `.bak`-Backups zurückgespielt sind.

Aufräum-Assertions (links/Task) laufen am Ende automatisch; Exitcode `0` =
grün.

### Testkonto-Hinweis

Für `T5` (Symlink ohne Dev-Mode) in einer VM **ohne** Developer Mode laufen
lassen: `New-FileSymlinkOrCopy` muss auf `Mode='copy'` zurückfallen, ohne
Fehler.

## Layer 3 — interaktive Abnahme (echte Hardware, 26100)

Nach `install.ps1 -Yes` (und Logon) auf dem Zielsystem:

- **Autostart:** Nach Logon läuft `aid.exe` (Task Scheduler). Prüfen:
  `Get-Process aid`.
- **Alt + ^ durch alle 4 Zustände:**
  1. kein Fenster → Alacritty + `herdr` starten, Monitor 2, opencode-Pane
     fokussiert.
  2. Fenster da, nicht Vordergrund → nach vorn holen + opencode-Pane.
  3. Fenster Vordergrund, opencode-Pane **nicht** fokussiert → nur Pane
     fokussieren.
  4. Fenster Vordergrund **und** opencode-Pane fokussiert → minimieren.
- **Herdr ohne Server:** erster Alt+^ bei fehlendem Herdr-Server → robuster
  Start, Retry laut `herdr_retry_*`.
- **Keine opencode-Session:** Alt+^ startet `opencode --auto` in einer neuen
  Pane (`pane split` + `agent start`).
- **Alacritty:** randlos, CaskaydiaCove Nerd Font, Transparenz akzeptabel
  (Fallback: deckend), `Ctrl+Shift+Space` = Vi-Mode/Copy.
- **LazyVim:** Markdown ohne Diagnostics, `<leader>ud` toggelt;
  `:set spelllang?` → `en,de`.
- **opencode:** Theme `ai-transparent` aktiv, MCPs exa + playwright vorhanden.

## Fehlersuche

- Journal: `install.log` im Repo-Root (bzw. `uninstall.log`).
- Daemon-Log: `%LOCALAPPDATA%\Lars-Win-AI\aid.log`.
- Bei fehlenden `config/**`-Assets: Komponenten melden `skipped` mit klarer
  Meldung (kein Crash); nach Bereitstellung der Assets erneut `install.ps1`.
