//go:build !windows

// Package platform: Nicht-Windows-Stubs, damit `go build ./...` auf jeder
// Plattform kompiliert. Der Daemon ist funktional nur unter Windows nutzbar.
package platform

import (
	"context"
	"errors"
	"os/exec"

	"github.com/Winkelars/Lars-Win-AI/internal/hotkey"
	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

// NewWindowManager liefert einen Manager ohne Fenster.
func NewWindowManager() window.Manager { return &unsupportedManager{} }

type unsupportedManager struct{}

func (m *unsupportedManager) FindByProcess(string) (window.Window, bool) { return nil, false }
func (m *unsupportedManager) FindByTitle(string) (window.Window, bool)   { return nil, false }
func (m *unsupportedManager) Enumerate() ([]window.Window, error)        { return nil, nil }

// NewHook liefert einen Hook, der unter Nicht-Windows einen Fehler meldet.
func NewHook(cfg hotkey.Config) hotkey.Hook { return &unsupportedHook{cfg: cfg} }

type unsupportedHook struct{ cfg hotkey.Config }

func (h *unsupportedHook) Run(ctx context.Context, events chan<- hotkey.Event) error {
	return errors.New("der Tastatur-Hook wird nur unter Windows unterstützt")
}

// AnimationsEnabled ist unter Nicht-Windows ein No-op.
func AnimationsEnabled() (bool, error) { return false, nil }

// SetAnimations ist unter Nicht-Windows ein No-op.
func SetAnimations(bool) error { return nil }

// ProcessAlive ist unter Nicht-Windows nicht verfügbar.
func ProcessAlive(int) bool { return false }

// HideConsole ist unter Nicht-Windows ein No-op.
func HideConsole(*exec.Cmd) {}

var (
	_ window.Manager = (*unsupportedManager)(nil)
	_ hotkey.Hook    = (*unsupportedHook)(nil)
)
