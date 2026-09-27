// Package window beschreibt die Fenster-Steuerung hinter testbaren
// Interfaces (docs/CONTRACTS.md §6). Die echte Windows-Implementierung
// liegt in internal/platform.
package window

import "strings"

// Window ist ein einzelnes steuerbares Top-Level-Fenster.
type Window interface {
	Handle() uintptr
	Title() string
	ProcessName() string
	IsForeground() bool
	Restore() error
	Minimize() error
	ShowNormal() error
	Focus() error
	MoveAndMaximize(monitor int) error
}

// Manager sucht Fenster.
type Manager interface {
	FindByProcess(processName string) (Window, bool)
	FindByTitle(title string) (Window, bool)
	Enumerate() ([]Window, error)
}

// TitleMatches prüft, ob ein Fenstertitel den gesuchten Titel enthält
// (case-insensitiv).
func TitleMatches(title, want string) bool {
	if strings.TrimSpace(want) == "" {
		return false
	}
	return strings.Contains(strings.ToLower(title), strings.ToLower(want))
}

// ProcessMatches vergleicht Prozessnamen case-insensitiv und ignoriert eine
// optionale .exe-Endung.
func ProcessMatches(process, want string) bool {
	if strings.TrimSpace(want) == "" {
		return false
	}
	p := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(process)), ".exe")
	w := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(want)), ".exe")
	return p == w
}

// Find sucht zuerst über den Titel und fällt dann auf den Prozessnamen zurück.
func Find(m Manager, title, process string) (Window, bool) {
	if m == nil {
		return nil, false
	}
	if strings.TrimSpace(title) != "" {
		if w, ok := m.FindByTitle(title); ok {
			return w, true
		}
	}
	if strings.TrimSpace(process) != "" {
		if w, ok := m.FindByProcess(process); ok {
			return w, true
		}
	}
	return nil, false
}
