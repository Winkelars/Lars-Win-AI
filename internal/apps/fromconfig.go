package apps

import (
	"strings"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
)

// Specs baut die Slot-Liste aus der Config: Herdr zuerst, dann die Apps.
func Specs(cfg *config.Config) []Spec {
	out := make([]Spec, 0, len(cfg.Apps)+1)
	out = append(out, Herdr(cfg))
	for _, a := range cfg.Apps {
		out = append(out, FromConfig(cfg, a))
	}
	return out
}

// Herdr baut den Herdr-Slot aus den globalen Feldern.
func Herdr(cfg *config.Config) Spec {
	launch := []string{cfg.WeztermPath}
	if strings.TrimSpace(cfg.WeztermConfig) != "" {
		launch = append(launch, "--config-file", cfg.WeztermConfig)
	}
	launch = append(launch, "start", "--", "herdr")
	return Spec{
		ID:       HerdrID,
		Kind:     HerdrID,
		ScanCode: cfg.Hotkey.ScanCode,
		Launch:   launch,
		Process:  cfg.ProcessName,
		Title:    cfg.WindowTitle,
		Monitor:  cfg.Monitor,
	}
}

// FromConfig wandelt einen Config-Slot in einen Spec.
func FromConfig(cfg *config.Config, a config.App) Spec {
	return Spec{
		ID:       a.ID,
		Kind:     "app",
		ScanCode: a.ScanCode,
		Launch:   cfg.ExpandArgs(a.Launch),
		Process:  a.Process,
		Class:    a.Class,
		Title:    a.Title,
		Monitor:  cfg.EffectiveMonitor(a),
		Alpha:    a.Alpha,
	}
}
