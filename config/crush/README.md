# config/crush — Crush / Herdr-Assets (WS-C)

Dieser Ordner enthält die Crush-Assets. Er ist die Quelle für den Installer (WS-I);
hier wird **nichts** systemseitig installiert.

## Inhalt

| Datei | Zweck |
|---|---|
| `skill/SKILL.md` | 1:1-Kopie von `herdr --skill` (YAML-Frontmatter + Body). |
| `README.md` | Diese Datei. |

## Native Herdr-Erkennung (kein eigener Reporter nötig)

Crush bringt die Herdr-Anbindung **selbst** mit (Paket `internal/herdr`). Läuft Crush
in einer Herdr-Pane (`HERDR_ENV=1`), meldet Crush Agent-Zustand (`idle`/`working`/
`blocked`) und Session-ID über den Herdr-Socket (Source `crush`, Agent `crush`) und
schickt beim Beenden `pane.release_agent`. Herdr zeigt die Pane dadurch automatisch
als **Agent-Pane** an. Herdr listet Crush entsprechend unter „supported by the agent —
nothing to install".

Deshalb gibt es bewusst **kein** `herdr integration install crush`; dieser Ordner
liefert nur den **Skill**.

Nachweis lokal (best-effort): `crush.exe` (0.96.1) enthält die Strings `HERDR_ENV`,
`HERDR_PANE_ID`, `HERDR_SOCKET_PATH`, `pane.report_agent`, `pane.release_agent`.

## Zielpfad und Installer-Verhalten

### Skill

- Junction: `config/crush/skill` → `%USERPROFILE%/.config/crush/skills/herdr`.
- Crush lädt Skills global aus `%USERPROFILE%\.config\crush\skills` automatisch
  (`crush/internal/config.GlobalSkillsDirs`); der Junction fügt `herdr` neben
  bestehenden Skills (email, photon-*, …) ein.
- Inhalt = exakte Ausgabe von `herdr --skill` (UTF-8 **ohne BOM**).
- Ein vorhandenes Ziel wird vorher gesichert (`*.bak`) und ersetzt; die Aktion ist
  idempotent (Junction-Erkennung über `New-JunctionSafe`).

## Pflege

- `SKILL.md` neu erzeugen: `herdr --skill` ausführen und die Ausgabe 1:1 speichern
  (UTF-8 **ohne BOM**, sonst beginnt die Datei nicht mit `---`). Vorlage/Quelle:
  `config/opencode/skill/SKILL.md` (agent-agnostisch).
- Bei Schema-Änderungen an Herdrs Skill-Format die Repo-Kopie aktualisieren.

## Verifikation

- Skill: beginnt mit `---` und enthält `name: herdr`.
- In einer Herdr-Pane `crush` starten; `herdr agent list` zeigt einen Agent `crush`
  mit wechselndem `agent_status`.
