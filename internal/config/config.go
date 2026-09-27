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

// Config ist das fixe Daemon-Config-Schema aus CONTRACTS §5.
type Config struct {
	WindowTitle      string   `json:"window_title"`
	ProcessName      string   `json:"process_name"`
	Monitor          int      `json:"monitor"`
	AgentName        string   `json:"agent_name"`
	AgentKind        string   `json:"agent_kind"`
	AgentArgs        []string `json:"agent_args"`
	WeztermPath      string   `json:"wezterm_path"`
	WeztermConfig    string   `json:"wezterm_config"`
	HerdrPath        string   `json:"herdr_path"`
	PaneStateFile    string   `json:"pane_state_file"`
	Hotkey           Hotkey   `json:"hotkey"`
	Animations       bool     `json:"animations"`
	StartupTimeoutMS int      `json:"startup_timeout_ms"`
	HerdrRetryMS     int      `json:"herdr_retry_ms"`
	HerdrRetryCount  int      `json:"herdr_retry_count"`

	// Path ist der tatsächlich geladene Config-Pfad (nicht serialisiert).
	Path string `json:"-"`
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
		Animations:       true,
		StartupTimeoutMS: 8000,
		HerdrRetryMS:     500,
		HerdrRetryCount:  40,
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
	return nil
}

// PaneStatePath liefert den expandierten Pane-State-Pfad.
func (c *Config) PaneStatePath() string { return ExpandEnv(c.PaneStateFile) }

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
