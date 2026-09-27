// Package runstate verwaltet eine einfache Single-Instance-Sperre über eine
// PID-Datei (%LOCALAPPDATA%\Lars-Win-AI\aid.pid).
package runstate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Winkelars/Lars-Win-AI/internal/platform"
)

// DefaultPath liefert %LOCALAPPDATA%\Lars-Win-AI\aid.pid.
func DefaultPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "Lars-Win-AI", "aid.pid")
}

// Acquire schreibt die aktuelle PID. Läuft bereits ein Daemon, wird ein Fehler
// zurückgegeben. Die zurückgegebene Funktion entfernt die PID-Datei.
func Acquire(path string) (func(), error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if pid, ok := readPID(path); ok && platform.ProcessAlive(pid) {
		return nil, fmt.Errorf("aid läuft bereits (PID %d)", pid)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}

// IsRunning prüft, ob laut PID-Datei ein lebender Daemon existiert.
func IsRunning(path string) bool {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	pid, ok := readPID(path)
	return ok && platform.ProcessAlive(pid)
}

func readPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}
