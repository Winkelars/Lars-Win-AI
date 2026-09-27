# Hyper-V E2E-Treiber (Host-seitig)

Steuert eine Hyper-V-VM **remote** (PowerShell Direct über VMBus, kein Netzwerk
nötig), führt den Installer-E2E im Guest aus und erstellt **Screenshots** vom
VM-Bildschirm zur visuellen Kontrolle.

## Voraussetzungen

- Host mit aktiviertem Hyper-V, laufende VM (z. B. „Windows 11-Entwicklungsumgebung").
- Der Treiber läuft **auf dem Host** und braucht **Administratorrechte**.
- Nur **Windows PowerShell 5.1** (nutzt `Get-WmiObject`/`GetRelated` für das VM-Thumbnail).
- Lokale **Admin-Zugangsdaten der VM** (PowerShell Direct).

## Aufruf

In einer **elevated** Windows PowerShell:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\e2e\vm\run-vm-e2e.ps1 `
  -VMName 'Windows 11-Entwicklungsumgebung' `
  -GuestUser User -GuestPassword 'DEIN_PASSWORT'
```

Parameter:

| Parameter | Default | Zweck |
|---|---|---|
| `-VMName` | Auto (einzige VM) | Ziel-VM. |
| `-GuestUser` / `-GuestPassword` | `User` / `Passw0rd!` | Lokale Admin-Zugangsdaten der VM. |
| `-RepoUrl` | GitHub-Repo | Wird im Guest geklont/gepullt. |
| `-GuestRepoPath` | `C:\Lars-Win-AI` | Arbeitsverzeichnis im Guest. |
| `-OutDir` | `tests/e2e/vm/out` | Ausgabeordner (je Lauf ein Zeitstempel-Unterordner). |
| `-SkipClone` | – | Repo nicht klonen/pullen. |
| `-SkipUninstall` | – | E2E ohne Uninstall-Schritt. |
| `-SkipE2E` | – | Nur Klonen + Screenshot (kein Install). |

## Ergebnis

Pro Lauf `tests/e2e/vm/out/<yyyyMMdd-HHmmss>/`:

- `driver.log` – Ablauf/Exit-Codes.
- `e2e-run.txt` – komplette Ausgabe von `tests/e2e/run.ps1` im Guest.
- `00-before.png`, `01-after.png` – Screenshots vor/nach dem Lauf.
- `summary.json` – VM, Exit-Code, Screenshot-Namen.

Exitcode `0` = E2E grün.

## Screenshot-Hinweis

Screenshots zeigen den **Konsolenbildschirm**. Ist die VM gesperrt (Lock-Screen),
siehst du den Lock-Screen; nach dem Login den Desktop. Für UI-Prüfungen der
Anwendung (WezTerm/opencode) vorher in der VM anmelden.

## Technik

- Remote-Steuerung: `New-PSSession -VMName … -Credential …` (PowerShell Direct).
- Screenshot: `Msvm_VirtualSystemManagementService.GetVirtualSystemThumbnailImage`
  (Namespace `root\virtualization\v2`) mit `Msvm_VirtualSystemSettingData` als
  Ziel; rohe RGB565-Daten → `System.Drawing` → PNG. Das `ComputerSystem` als Ziel
  liefert ein leeres Bild, deshalb explizit die SettingData verwenden.
