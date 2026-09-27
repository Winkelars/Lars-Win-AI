// Package toggle enthält die reine Toggle-Zustandsmaschine (Decide) sowie die
// ausführende Run-Funktion (CONTRACTS §6).
package toggle

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/herdr"
	"github.com/Winkelars/Lars-Win-AI/internal/platform"
	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

// Action beschreibt das Ergebnis einer Toggle-Entscheidung.
type Action int

const (
	// ActionNone bedeutet: nichts zu tun.
	ActionNone Action = iota
	// ActionStart: Fenster fehlt -> Alacritty + herdr starten.
	ActionStart
	// ActionForeground: Fenster existiert, ist aber nicht im Vordergrund.
	ActionForeground
	// ActionFocusPane: Fenster im Vordergrund, opencode-Pane nicht fokussiert.
	ActionFocusPane
	// ActionMinimize: Fenster im Vordergrund und opencode-Pane fokussiert.
	ActionMinimize
	// ActionFocusAndForeground wird der Vollständigkeit halber definiert.
	ActionFocusAndForeground
)

// String liefert eine lesbare Action-Bezeichnung.
func (a Action) String() string {
	switch a {
	case ActionStart:
		return "start"
	case ActionForeground:
		return "foreground"
	case ActionFocusPane:
		return "focus-pane"
	case ActionMinimize:
		return "minimize"
	case ActionFocusAndForeground:
		return "focus-and-foreground"
	default:
		return "none"
	}
}

// Deps bündelt die Abhängigkeiten der Zustandsmaschine.
type Deps struct {
	Windows window.Manager
	Herdr   herdr.Client
	CFG     *config.Config
}

// Decide ist die reine Zustandsmaschine (CONTRACTS §6): Fenster fehlt ->
// starten, nicht im Vordergrund -> nach vorn holen, im Vordergrund -> minimieren.
// Bewusst NICHT abhaengig vom herdr-Fokus-Flag, da dieses bei Headless-Aufrufen
// unzuverlaessig ist und den Toggle sonst "haengen" laesst.
func Decide(foreground, windowExists bool) Action {
	switch {
	case !windowExists:
		return ActionStart
	case !foreground:
		return ActionForeground
	default:
		return ActionMinimize
	}
}

// startProcess ist eine Naht für Tests (Alacritty-Start).
var startProcess = func(cmd *exec.Cmd) error { return cmd.Start() }

// Run ermittelt den Zustand und führt ihn real aus.
func Run(ctx context.Context, d Deps, cfg *config.Config) (Action, error) {
	if cfg == nil {
		cfg = d.CFG
	}
	if cfg == nil {
		return ActionNone, fmt.Errorf("toggle: keine Konfiguration")
	}
	if d.Windows == nil {
		return ActionNone, fmt.Errorf("toggle: kein Fenster-Manager")
	}

	win, exists := window.Find(d.Windows, cfg.WindowTitle, cfg.ProcessName)
	foreground := exists && win.IsForeground()

	action := Decide(foreground, exists)
	switch action {
	case ActionStart:
		if err := startAlacritty(cfg); err != nil {
			return action, err
		}
		w, ok := waitForWindow(ctx, d.Windows, cfg)
		if !ok {
			return action, fmt.Errorf("Fenster %q erschien nicht innerhalb %dms",
				cfg.WindowTitle, cfg.StartupTimeoutMS)
		}
		if err := w.MoveAndMaximize(cfg.Monitor); err != nil {
			return action, err
		}
		if err := w.Focus(); err != nil {
			return action, err
		}
		if err := focusOpencode(ctx, d, cfg); err != nil {
			return action, err
		}
	case ActionForeground:
		if err := win.Restore(); err != nil {
			return action, err
		}
		if err := win.MoveAndMaximize(cfg.Monitor); err != nil {
			return action, err
		}
		if err := win.Focus(); err != nil {
			return action, err
		}
		if err := focusOpencode(ctx, d, cfg); err != nil {
			return action, err
		}
	case ActionMinimize:
		if err := win.Minimize(); err != nil {
			return action, err
		}
	}
	return action, nil
}

func focusOpencode(ctx context.Context, d Deps, cfg *config.Config) error {
	if d.Herdr == nil {
		return nil
	}
	agents, err := d.Herdr.AgentList(ctx)
	if err == nil {
		if a, ok := herdr.FindAgent(agents, cfg.AgentName, cfg.AgentKind); ok {
			if target := a.Target(); target != "" {
				return d.Herdr.AgentFocus(ctx, target)
			}
		}
	}
	return startOpencode(ctx, d.Herdr, cfg)
}

func startOpencode(ctx context.Context, c herdr.Client, cfg *config.Config) error {
	paneID := ""
	if st, err := LoadPaneState(cfg.PaneStatePath()); err == nil && st != nil && st.PaneID != "" {
		if panes, err := c.PaneList(ctx); err == nil {
			for _, p := range panes {
				if p.ID == st.PaneID {
					paneID = p.ID
					break
				}
			}
		}
	}
	if paneID == "" {
		panes, err := c.PaneList(ctx)
		if err != nil {
			return fmt.Errorf("pane list: %w", err)
		}
		if len(panes) == 0 {
			return fmt.Errorf("kein Pane zum Splitten vorhanden")
		}
		id, err := c.PaneSplit(ctx, panes[0].ID, "right")
		if err != nil {
			return fmt.Errorf("pane split: %w", err)
		}
		paneID = id
	}
	if err := c.AgentStart(ctx, cfg.AgentName, cfg.AgentKind, paneID, cfg.AgentArgs); err != nil {
		return fmt.Errorf("agent start: %w", err)
	}
	_ = SavePaneState(cfg.PaneStatePath(), &PaneState{
		PaneID:    paneID,
		UpdatedAt: time.Now().UTC(),
	})
	return nil
}

func startAlacritty(cfg *config.Config) error {
	args := []string{"-T", cfg.WindowTitle}
	if cfg.AlacrittyConfig != "" {
		args = append(args, "--config-file", cfg.AlacrittyConfig)
	}
	args = append(args, "-e", "herdr")
	cmd := exec.Command(cfg.AlacrittyPath, args...)
	platform.HideConsole(cmd)
	if err := startProcess(cmd); err != nil {
		return fmt.Errorf("alacritty starten (%s): %w", cfg.AlacrittyPath, err)
	}
	return nil
}

func waitForWindow(ctx context.Context, m window.Manager, cfg *config.Config) (window.Window, bool) {
	deadline := time.Now().Add(time.Duration(cfg.StartupTimeoutMS) * time.Millisecond)
	for {
		if w, ok := window.Find(m, cfg.WindowTitle, cfg.ProcessName); ok {
			return w, true
		}
		if time.Now().After(deadline) {
			return nil, false
		}
		select {
		case <-ctx.Done():
			return nil, false
		case <-time.After(50 * time.Millisecond):
		}
	}
}
