# config/opencode — opencode-Assets (WS-O)

Dieser Ordner enthält die opencode-Assets aus `docs/CONTRACTS.md` §10. Er ist die
Quelle für den Installer (WS-I); hier wird **nichts** systemseitig installiert.

## Inhalt

| Datei | Zweck |
|---|---|
| `themes/ai-transparent.json` | opencode-Theme mit 100 % transparentem Hintergrund. |
| `mcp.snippet.jsonc` | JSONC-Fragment mit den MCP-Servern `exa` (remote) und `playwright` (local). |
| `skill/SKILL.md` | 1:1-Kopie von `herdr --skill` (YAML-Frontmatter + Body). |
| `README.md` | Diese Datei. |

## Zielpfade und Installer-Verhalten

Der Installer (WS-I) übernimmt die Assets wie folgt. Details sind in
`docs/CONTRACTS.md` §2 (Symlink-Tabelle) und §10 festgeschrieben.

### Theme

- Junction: `config/opencode/themes` → `%USERPROFILE%/.config/opencode/themes`.
- Aktivierung via `"theme": "ai-transparent"` in `opencode.jsonc` (Installer-Merge).
- Das Theme setzt `background`, `backgroundPanel`, `backgroundElement` und
  `backgroundMenu` auf `"none"`; weitere Hintergrund-Felder (Diff-Flächen,
  `selectedListItemText`) ebenfalls, damit die Terminal-Transparenz von
  WezTerm durchscheint.

### MCP-Merge

- `mcp.snippet.jsonc` wird **idempotent** in die bestehende
  `%USERPROFILE%/.config/opencode/opencode.jsonc` gemerged (Key `mcp`).
- **Kein Clobber:** bereits vorhandene Keys wie `zoho` (und die Brain-Reference)
  bleiben unverändert; `exa`/`playwright` werden nur ergänzt bzw. auf den
  Snippet-Wert aktualisiert.
- **Kommentare bleiben erhalten** (JSONC-Merge, nicht Neuschreiben).
- **Vorher `.bak`-Backup** der Zieldatei (`.bak.<ts>` via `Backup-Path`).
- Ferner setzt der Installer `EDITOR`/`VISUAL = nvim` und startet die
  opencode-Sitzung standardmäßig mit `--auto`.

### Skill

- Junction: `config/opencode/skill` → `%USERPROFILE%/.config/opencode/skills/herdr`
  (der Zielordner heißt `skills/herdr`, die Quelle bleibt `skill/`).
- Damit erkennt opencode den Herdr-Skill (`name: herdr` im Frontmatter).
- Inhalt = exakte Ausgabe von `herdr --skill` (UTF-8 ohne BOM).

## Pflege

- `SKILL.md` neu erzeugen: `herdr --skill` ausführen und die Ausgabe 1:1
  speichern (UTF-8 **ohne BOM**, sonst beginnt die Datei nicht mit `---`).
- Theme/Snippet bei Schema-Änderungen an `docs/CONTRACTS.md` §10 anpassen —
  der Vertrag ist die maßgebliche Referenz.

## Verifikation

- Theme: valides JSON (`Get-Content ... -Raw | ConvertFrom-Json`).
- Snippet: JSONC (Kommentare erlaubt) — mit einem JSONC-Parser oder nach
  Entfernen der Kommentare parsen.
- Skill: beginnt mit `---` und enthält `name: herdr`.
