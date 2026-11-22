// Package config lädt, ergänzt und validiert die Daemon-Konfiguration
// (docs/CONTRACTS.md §5). Alle Felder sind optional; Defaults greifen.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Hotkey beschreibt die Low-Level-Hook-Erkennung des Toggle-Hotkeys.
type Hotkey struct {
	ScanCode     uint32 `json:"scan_code"`
	AltOnly      bool   `json:"alt_only"`
	ExcludeAltGr bool   `json:"exclude_altgr"`
}

// App ist ein Fenster-Slot (CONTRACTS §5 apps[]).
type App struct {
	ID       string   `json:"id"`
	ScanCode uint32   `json:"scan_code"`
	Launch   []string `json:"launch"`
	Process  string   `json:"process"`
	Class    string   `json:"class"`
	Title    string   `json:"title"`
	Monitor  int      `json:"monitor"`
	Alpha    int      `json:"alpha"`
	Kind     string   `json:"kind"`
}

// Config ist das fixe Daemon-Config-Schema aus CONTRACTS §5.
type Config struct {
	WindowTitle   string   `json:"window_title"`
	ProcessName   string   `json:"process_name"`
	Monitor       int      `json:"monitor"`
	AgentName     string   `json:"agent_name"`
	AgentKind     string   `json:"agent_kind"`
	AgentArgs     []string `json:"agent_args"`
	WeztermPath   string   `json:"wezterm_path"`
	WeztermConfig string   `json:"wezterm_config"`
	HerdrPath     string   `json:"herdr_path"`
	PaneStateFile string   `json:"pane_state_file"`
	Hotkey        Hotkey   `json:"hotkey"`
	// AlphaHotkey schaltet die kuenstliche Transparenz aller Slots um.
	AlphaHotkey      Hotkey `json:"alpha_hotkey"`
	Animations       bool   `json:"animations"`
	StartupTimeoutMS int    `json:"startup_timeout_ms"`
	HerdrRetryMS     int    `json:"herdr_retry_ms"`
	HerdrRetryCount  int    `json:"herdr_retry_count"`
	Apps             []App  `json:"apps"`

	// Path ist der tatsächlich geladene Config-Pfad (nicht serialisiert).
	Path string `json:"-"`
}

// DefaultApps liefert die Standard-Slots (Alt+1 bis Alt+8). Herdr selbst ist
// kein Eintrag: der Slot wird aus window_title/process_name/hotkey gebildet.
func DefaultApps() []App {
	return []App{
		{
			ID: "terminal", ScanCode: 2, Process: "WindowsTerminal.exe", Class: "CASCADIA_HOSTING_WINDOW_CLASS",
			Launch: []string{"wt.exe"}, Monitor: 2,
		},
		{
			ID: "explorer", ScanCode: 3, Process: "explorer.exe", Class: "CabinetWClass",
			Launch: []string{"explorer.exe"}, Monitor: 2, Alpha: 235,
		},
		{
			ID: "vivaldi", ScanCode: 4, Process: "vivaldi.exe", Class: "Chrome_WidgetWin_1",
			Launch: []string{`%LOCALAPPDATA%\Vivaldi\Application\vivaldi.exe`}, Monitor: 2, Alpha: 235,
		},
		{
			ID: "discord", ScanCode: 5, Process: "Discord.exe", Class: "Chrome_WidgetWin_1",
			Launch: []string{`%LOCALAPPDATA%\Discord\Update.exe`, "--processStart", "Discord.exe"}, Monitor: 2, Alpha: 235,
		},
		{
			ID: "ytmusic", ScanCode: 6, Process: "YouTube Music.exe", Class: "Chrome_WidgetWin_1",
			Launch: []string{`%LOCALAPPDATA%\Programs\youtube-music\YouTube Music.exe`}, Monitor: 2,
		},
		{
			ID: "audiobook", ScanCode: 7, Process: "chrome.exe", Class: "Chrome_WidgetWin_1", Title: "Audiobookshelf",
			Launch: []string{`%LOCALAPPDATA%\Chromium\Application\chrome_proxy.exe`,
				"--profile-directory=Default", "--app-id=milnkbghnijmlofofbkeajbckianfiec"}, Monitor: 2, Alpha: 235,
		},
		{
			ID: "deepseek", ScanCode: 8, Process: "chrome.exe", Class: "Chrome_WidgetWin_1", Title: "DeepSeek",
			Launch: []string{`%LOCALAPPDATA%\Chromium\Application\chrome_proxy.exe`,
				"--profile-directory=Default", "--app-id=obodlahpolhdfoaohcfenfnabipjmedh"}, Monitor: 2, Alpha: 235,
		},
		{
			ID: "steam", ScanCode: 9, Process: "steam.exe", Class: "vguiPopupWindow",
			Launch: []string{`C:\Program Files (x86)\Steam\steam.exe`}, Monitor: 2, Alpha: 235,
		},
	}
}

// Defaults liefert die vollständige Konfiguration mit allen Vorgabewerten.
func Defaults() *Config {
	return &Config{
		WindowTitle:      "AI-Assistant",
		ProcessName:      "wezterm-gui.exe",
		Monitor:          2,
		AgentName:        "opencode",
		AgentKind:        "opencode",
		AgentArgs:        []string{"--auto"},
		WeztermPath:      `C:\Program Files\WezTerm\wezterm.exe`,
		WeztermConfig:    "",
		HerdrPath:        "herdr",
		PaneStateFile:    `%APPDATA%\Lars-Win-AI\pane-state.json`,
		Hotkey:           Hotkey{ScanCode: 41, AltOnly: true, ExcludeAltGr: true},
		AlphaHotkey:      Hotkey{ScanCode: 20, AltOnly: true, ExcludeAltGr: true}, // Alt+T
		Animations:       true,
		StartupTimeoutMS: 8000,
		HerdrRetryMS:     500,
		HerdrRetryCount:  40,
		Apps:             DefaultApps(),
	}
}

// DefaultPath liefert den Standard-Config-Pfad (%APPDATA%\Lars-Win-AI\config.json).
// Der Env-Override AID_CONFIG gewinnt.
func DefaultPath() string {
	if v := strings.TrimSpace(os.Getenv("AID_CONFIG")); v != "" {
		return ExpandEnv(v)
	}
	return filepath.Join(appData(), "Lars-Win-AI", "config.json")
}

// Load lädt die Konfiguration. path == "" verwendet DefaultPath().
// Eine fehlende Datei ist kein Fehler: Defaults bleiben bestehen.
func Load(path string) (*Config, error) {
	cfg := Defaults()
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		resolved = DefaultPath()
	}
	resolved = ExpandEnv(resolved)

	data, err := os.ReadFile(resolved)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config %s: %w", resolved, err)
		}
	case os.IsNotExist(err):
		// Defaults verwenden.
	default:
		return nil, fmt.Errorf("config lesen %s: %w", resolved, err)
	}

	applyEnv(cfg)
	cfg.Path = resolved
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", resolved, err)
	}
	return cfg, nil
}

// Validate prüft die Konfiguration auf offensichtliche Fehler.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.WindowTitle) == "" {
		return fmt.Errorf("window_title darf nicht leer sein")
	}
	if strings.TrimSpace(c.ProcessName) == "" {
		return fmt.Errorf("process_name darf nicht leer sein")
	}
	if c.Monitor < 1 {
		return fmt.Errorf("monitor muss >= 1 sein (ist %d)", c.Monitor)
	}
	if strings.TrimSpace(c.AgentName) == "" {
		return fmt.Errorf("agent_name darf nicht leer sein")
	}
	if c.StartupTimeoutMS <= 0 {
		return fmt.Errorf("startup_timeout_ms muss > 0 sein (ist %d)", c.StartupTimeoutMS)
	}
	if c.HerdrRetryMS < 0 {
		return fmt.Errorf("herdr_retry_ms darf nicht negativ sein (ist %d)", c.HerdrRetryMS)
	}
	if c.HerdrRetryCount < 0 {
		return fmt.Errorf("herdr_retry_count darf nicht negativ sein (ist %d)", c.HerdrRetryCount)
	}
	if c.Hotkey.ScanCode == 0 {
		return fmt.Errorf("hotkey.scan_code darf nicht 0 sein")
	}
	if c.AlphaHotkey.ScanCode == 0 {
		return fmt.Errorf("alpha_hotkey.scan_code darf nicht 0 sein")
	}
	if c.AlphaHotkey.ScanCode == c.Hotkey.ScanCode {
		return fmt.Errorf("alpha_hotkey darf nicht denselben ScanCode wie hotkey haben")
	}
	if err := c.validateApps(); err != nil {
		return err
	}
	return nil
}

// PaneStatePath liefert den expandierten Pane-State-Pfad.
func (c *Config) PaneStatePath() string { return ExpandEnv(c.PaneStateFile) }

// EffectiveMonitor liefert den Monitor eines Slots. 0 im Slot bedeutet: der
// globale monitor-Wert gilt.
func (c *Config) EffectiveMonitor(a App) int {
	if a.Monitor >= 1 {
		return a.Monitor
	}
	if c.Monitor >= 1 {
		return c.Monitor
	}
	return 1
}

// ExpandArgs expandiert %VAR% und $VAR in Launch-Argumenten.
func (c *Config) ExpandArgs(argv []string) []string {
	out := make([]string, 0, len(argv))
	for _, arg := range argv {
		out = append(out, ExpandEnv(arg))
	}
	return out
}

// LookupApp sucht einen Slot über die ID.
func (c *Config) LookupApp(id string) (App, bool) {
	for _, a := range c.Apps {
		if a.ID == id {
			return a, true
		}
	}
	return App{}, false
}

// validateApps prüft die Slot-Liste auf Eindeutigkeit und Plausibilität.
func (c *Config) validateApps() error {
	seenID := make(map[string]string, len(c.Apps))
	seenScan := make(map[uint32]string, len(c.Apps)+1)
	if c.Hotkey.ScanCode != 0 {
		seenScan[c.Hotkey.ScanCode] = "herdr"
	}
	if c.AlphaHotkey.ScanCode != 0 {
		seenScan[c.AlphaHotkey.ScanCode] = "alpha"
	}
	for _, a := range c.Apps {
		id := strings.TrimSpace(a.ID)
		if id == "" {
			return fmt.Errorf("apps: id darf nicht leer sein")
		}
		if id == "herdr" {
			return fmt.Errorf("apps: id %q ist für den Herdr-Slot reserviert", id)
		}
		if id == "alpha" {
			return fmt.Errorf("apps: id %q ist für den Transparenz-Toggle reserviert", id)
		}
		if other, ok := seenID[id]; ok {
			return fmt.Errorf("apps: id %q doppelt", other)
		}
		seenID[id] = id
		if a.ScanCode == 0 {
			return fmt.Errorf("apps[%s]: scan_code darf nicht 0 sein", id)
		}
		if other, ok := seenScan[a.ScanCode]; ok {
			return fmt.Errorf("apps[%s]: scan_code %d ist bereits von %s belegt", id, a.ScanCode, other)
		}
		seenScan[a.ScanCode] = id
		if strings.TrimSpace(a.Process) == "" && strings.TrimSpace(a.Title) == "" && strings.TrimSpace(a.Class) == "" {
			return fmt.Errorf("apps[%s]: process, class oder title muss gesetzt sein", id)
		}
		if a.Monitor < 0 {
			return fmt.Errorf("apps[%s]: monitor darf nicht negativ sein (ist %d)", id, a.Monitor)
		}
		if a.Alpha < 0 || a.Alpha > 255 {
			return fmt.Errorf("apps[%s]: alpha muss zwischen 0 und 255 liegen (ist %d)", id, a.Alpha)
		}
		switch a.Kind {
		case "", "app", "herdr":
		default:
			return fmt.Errorf("apps[%s]: unbekannte kind %q", id, a.Kind)
		}
		for _, arg := range a.Launch {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("apps[%s]: launch enthält ein leeres Argument", id)
			}
		}
	}
	return nil
}

func applyEnv(c *Config) {
	if v := os.Getenv("AID_WINDOW_TITLE"); v != "" {
		c.WindowTitle = v
	}
	if v := strings.TrimSpace(os.Getenv("AID_MONITOR")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Monitor = n
		}
	}
	if v := os.Getenv("AID_AGENT_NAME"); v != "" {
		c.AgentName = v
	}
	if v := os.Getenv("AID_AGENT_KIND"); v != "" {
		c.AgentKind = v
	}
	if v := os.Getenv("AID_AGENT_ARGS"); strings.TrimSpace(v) != "" {
		c.AgentArgs = strings.Fields(v)
	}
	if v := os.Getenv("AID_WEZTERM_PATH"); v != "" {
		c.WeztermPath = v
	}
	if v := os.Getenv("AID_WEZTERM_CONFIG"); v != "" {
		c.WeztermConfig = v
	}
	if v := os.Getenv("AID_HERDR_PATH"); v != "" {
		c.HerdrPath = v
	}
}

var percentVar = regexp.MustCompile(`%([^%]+)%`)

// ExpandEnv expandiert sowohl %VAR% (Windows) als auch $VAR/${VAR}.
// Unbekannte %VAR% bleiben unverändert.
func ExpandEnv(s string) string {
	s = percentVar.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return m
	})
	return os.ExpandEnv(s)
}

func appData() string {
	if v := os.Getenv("APPDATA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Roaming")
}
