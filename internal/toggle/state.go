package toggle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
)

// PaneState ist der persistierte, validierte designierte opencode-Pane
// (CONTRACTS §5 pane_state_file).
type PaneState struct {
	PaneID      string    `json:"pane_id"`
	TabID       string    `json:"tab_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// LoadPaneState liest die State-Datei. Eine fehlende Datei oder ein leerer
// Pfad sind kein Fehler (nil, nil).
func LoadPaneState(path string) (*PaneState, error) {
	if path == "" {
		return nil, nil
	}
	resolved := config.ExpandEnv(path)
	data, err := os.ReadFile(resolved)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st PaneState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// SavePaneState schreibt die State-Datei (legt das Verzeichnis an).
func SavePaneState(path string, st *PaneState) error {
	if path == "" || st == nil {
		return nil
	}
	resolved := config.ExpandEnv(path)
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(resolved, append(data, '\n'), 0o644)
}
