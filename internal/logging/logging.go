// Package logging schreibt Daemon-Logs in eine Datei
// (%LOCALAPPDATA%\Lars-Win-AI\aid.log) und gibt bewusst nichts auf der
// Konsole aus (der Daemon läuft als windowsgui-Binary).
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level beschreibt die Log-Schwere.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String liefert das kürzel des Levels.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// ParseLevel wandelt einen String in einen Level (Default: Info).
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger ist ein nebenläufig nutzbarer Datei-Logger.
type Logger struct {
	mu    sync.Mutex
	w     io.Writer
	f     *os.File
	level Level
}

// DefaultPath liefert %LOCALAPPDATA%\Lars-Win-AI\aid.log.
func DefaultPath() string {
	return filepath.Join(localAppData(), "Lars-Win-AI", "aid.log")
}

// New öffnet (bzw. erstellt) die Log-Datei.
func New(path string, level Level) (*Logger, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{w: f, f: f, level: level}, nil
}

// Nop liefert einen Logger, der nichts schreibt.
func Nop() *Logger { return &Logger{w: io.Discard, level: Level(99)} }

// Close schließt die Log-Datei.
func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.f.Close()
	l.f = nil
	return err
}

// Debugf loggt auf Debug-Level.
func (l *Logger) Debugf(format string, args ...interface{}) { l.logf(LevelDebug, format, args...) }

// Infof loggt auf Info-Level.
func (l *Logger) Infof(format string, args ...interface{}) { l.logf(LevelInfo, format, args...) }

// Warnf loggt auf Warn-Level.
func (l *Logger) Warnf(format string, args ...interface{}) { l.logf(LevelWarn, format, args...) }

// Errorf loggt auf Error-Level.
func (l *Logger) Errorf(format string, args ...interface{}) { l.logf(LevelError, format, args...) }

func (l *Logger) logf(level Level, format string, args ...interface{}) {
	if l == nil || level < l.level {
		return
	}
	ts := time.Now().Format("2006-01-02T15:04:05.000")
	line := fmt.Sprintf("%s [%s] %s\n", ts, level.String(), fmt.Sprintf(format, args...))
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.w, line)
}

func localAppData() string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local")
}
