# Herdr-Assets (WS-A)

Diese Datei dokumentiert Annahmen und Validierung der Herdr-Konfiguration
`config/herdr/config.toml`. Vertrag: `docs/CONTRACTS.md` §8.

## Inhalt

- `onboarding = false` — kein Erststart-Onboarding.
- `[ui] host_cursor = "native"` — nativer Host-Cursor (Bestandswert, beibehalten).
- `[ui] window_title = "AI-Assistant"` — stabiler OS-Fenstertitel, über den der
  Go-Daemon das Alacritty-/Herdr-Fenster findet.
- `[theme] name = "terminal"` + `[theme.custom] panel_bg = "reset"` — transparenter
  Panel-Hintergrund passend zu Alacritty (siehe unten).

## Getroffene Annahme: transparenter Panel-Hintergrund

Das Theme-Schema wurde über `herdr --default-config` (herdr
0.9.1-preview.2026-09-21) recherchiert. Für `[theme.custom]` dokumentiert die
Default-Config als akzeptierte Werte ausschließlich:

- Hex (`#rrggbb`), benannte Farben, `rgb(r,g,b)`
- der Sonderwert `panel_bg = "reset"`

Ein explizites Alpha-/Transparenz-Token (z. B. `transparent = true` oder 8-stelliges
`#RRGGBBAA`) existiert **nicht**. `herdr config check` validiert zudem nur die
TOML-Struktur, nicht die Farbwerte.

**Gewählter, konservativer Weg:** Built-in-Theme `terminal` (erbt die Farben des
äußeren Terminals) plus `panel_bg = "reset"`. `reset` setzt den Panel-Hintergrund
auf den Terminal-Standard zurück; im äußeren Alacritty mit `opacity = 0.90` und
`background = "#000000"` scheint dadurch der Desktop durch das Panel. Das ist die
nächstliegende, garantiert gültige Umsetzung von „transparentem Panel-BG“.

Falls eine spätere Herdr-Version ein echtes Transparenz-Token einführt, ist dies
hier zu aktualisieren.

## Validierung

```powershell
# Herdr prüft die Repo-Datei (Env-Override, ändert nichts am System):
$env:HERDR_CONFIG_PATH = "$PWD\config\herdr\config.toml"
herdr config check
# -> config: ok
```

```powershell
# TOML-Syntaxprüfung über den Node-Parser:
node -e "const t=require('C:/Users/Makkis/.config/opencode/node_modules/toml'); try{ t.parse(require('fs').readFileSync(process.argv[1],'utf8')); console.log('TOML OK'); }catch(e){ console.error('TOML FAIL: '+e.message); process.exit(1);}" config/herdr/config.toml
```
