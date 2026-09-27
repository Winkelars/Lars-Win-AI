package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearAidEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AID_WINDOW_TITLE", "AID_MONITOR", "AID_AGENT_NAME", "AID_AGENT_KIND",
		"AID_AGENT_ARGS", "AID_WEZTERM_PATH", "AID_WEZTERM_CONFIG",
		"AID_HERDR_PATH", "AID_CONFIG",
	} {
		t.Setenv(key, "")
	}
}

func TestDefaults(t *testing.T) {
	c := Defaults()
	if c.WindowTitle != "AI-Assistant" {
		t.Errorf("WindowTitle = %q", c.WindowTitle)
	}
	if c.ProcessName != "wezterm-gui.exe" {
		t.Errorf("ProcessName = %q", c.ProcessName)
	}
	if c.Monitor != 2 {
		t.Errorf("Monitor = %d", c.Monitor)
	}
	if c.AgentName != "opencode" || c.AgentKind != "opencode" {
		t.Errorf("Agent = %q/%q", c.AgentName, c.AgentKind)
	}
	if len(c.AgentArgs) != 1 || c.AgentArgs[0] != "--auto" {
		t.Errorf("AgentArgs = %v", c.AgentArgs)
	}
	if c.HerdrPath != "herdr" {
		t.Errorf("HerdrPath = %q", c.HerdrPath)
	}
	if c.Hotkey.ScanCode != 41 || !c.Hotkey.AltOnly || !c.Hotkey.ExcludeAltGr {
		t.Errorf("Hotkey = %+v", c.Hotkey)
	}
	if c.StartupTimeoutMS != 8000 || c.HerdrRetryMS != 500 || c.HerdrRetryCount != 40 {
		t.Errorf("Timeouts = %d/%d/%d", c.StartupTimeoutMS, c.HerdrRetryMS, c.HerdrRetryCount)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Defaults ungültig: %v", err)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	clearAidEnv(t)
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.WindowTitle != "AI-Assistant" || cfg.Monitor != 2 {
		t.Fatalf("Defaults nicht angewendet: %+v", cfg)
	}
}

func TestLoadMergesDefaultsWithFile(t *testing.T) {
	clearAidEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
	  "monitor": 3,
	  "agent_args": ["--auto", "--model", "x"],
	  "hotkey": {"scan_code": 42}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Monitor != 3 {
		t.Errorf("Monitor = %d, erwartet 3", cfg.Monitor)
	}
	if len(cfg.AgentArgs) != 3 || cfg.AgentArgs[1] != "--model" {
		t.Errorf("AgentArgs = %v", cfg.AgentArgs)
	}
	if cfg.Hotkey.ScanCode != 42 {
		t.Errorf("ScanCode = %d, erwartet 42", cfg.Hotkey.ScanCode)
	}
	// Nicht gesetzte Felder behalten Defaults.
	if !cfg.Hotkey.AltOnly || !cfg.Hotkey.ExcludeAltGr {
		t.Errorf("Hotkey-Defaults verloren: %+v", cfg.Hotkey)
	}
	if cfg.WindowTitle != "AI-Assistant" || cfg.ProcessName != "wezterm-gui.exe" {
		t.Errorf("Feld-Defaults verloren: %+v", cfg)
	}
	if cfg.Path != path {
		t.Errorf("Path = %q, erwartet %q", cfg.Path, path)
	}
}

func TestEnvOverrides(t *testing.T) {
	clearAidEnv(t)
	t.Setenv("AID_WINDOW_TITLE", "FromEnv")
	t.Setenv("AID_MONITOR", "4")
	t.Setenv("AID_AGENT_NAME", "myagent")
	t.Setenv("AID_AGENT_KIND", "opencode")
	t.Setenv("AID_AGENT_ARGS", "--auto --verbose")
	t.Setenv("AID_WEZTERM_PATH", `D:\wezterm.exe`)
	t.Setenv("AID_WEZTERM_CONFIG", `D:\wezterm.lua`)
	t.Setenv("AID_HERDR_PATH", `D:\herdr.exe`)

	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"window_title":"FromFile","monitor":1,"agent_name":"fileagent"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.WindowTitle != "FromEnv" {
		t.Errorf("WindowTitle = %q", cfg.WindowTitle)
	}
	if cfg.Monitor != 4 {
		t.Errorf("Monitor = %d", cfg.Monitor)
	}
	if cfg.AgentName != "myagent" || cfg.AgentKind != "opencode" {
		t.Errorf("Agent = %q/%q", cfg.AgentName, cfg.AgentKind)
	}
	if len(cfg.AgentArgs) != 2 || cfg.AgentArgs[0] != "--auto" || cfg.AgentArgs[1] != "--verbose" {
		t.Errorf("AgentArgs = %v", cfg.AgentArgs)
	}
	if cfg.WeztermPath != `D:\wezterm.exe` || cfg.WeztermConfig != `D:\wezterm.lua` {
		t.Errorf("Wezterm = %q / %q", cfg.WeztermPath, cfg.WeztermConfig)
	}
	if cfg.HerdrPath != `D:\herdr.exe` {
		t.Errorf("HerdrPath = %q", cfg.HerdrPath)
	}
}

func TestAIDConfigOverride(t *testing.T) {
	clearAidEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "via-env.json")
	if err := os.WriteFile(path, []byte(`{"monitor": 9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AID_CONFIG", path)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Monitor != 9 {
		t.Errorf("Monitor = %d, erwartet 9", cfg.Monitor)
	}
	if cfg.Path != path {
		t.Errorf("Path = %q", cfg.Path)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"leerer Titel", func(c *Config) { c.WindowTitle = " " }},
		{"leerer Prozess", func(c *Config) { c.ProcessName = "" }},
		{"Monitor 0", func(c *Config) { c.Monitor = 0 }},
		{"leerer Agent", func(c *Config) { c.AgentName = "" }},
		{"Timeout 0", func(c *Config) { c.StartupTimeoutMS = 0 }},
		{"Retry negativ", func(c *Config) { c.HerdrRetryMS = -1 }},
		{"RetryCount negativ", func(c *Config) { c.HerdrRetryCount = -1 }},
		{"ScanCode 0", func(c *Config) { c.Hotkey.ScanCode = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Defaults()
			tc.mutate(c)
			if err := c.Validate(); err == nil {
				t.Fatalf("Validate(): erwartet Fehler, bekam nil")
			}
		})
	}
}

func TestExpandEnvPercent(t *testing.T) {
	t.Setenv("MYVAR", "hello")
	if got := ExpandEnv(`%MYVAR%\sub`); got != `hello\sub` {
		t.Errorf("ExpandEnv = %q", got)
	}
	if got := ExpandEnv(`%UNSET_VAR_XYZ%`); got != `%UNSET_VAR_XYZ%` {
		t.Errorf("ExpandEnv unset = %q", got)
	}
}
