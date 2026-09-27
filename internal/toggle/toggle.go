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
	// Logf ist optional und dient der Diagnose (z. B. log.Debugf).
	Logf func(format string, args ...interface{})
}

func (d Deps) logf(format string, args ...interface{}) {
	if d.Logf != nil {
		d.Logf(format, args...)
	}
}

// Decide ist die reine Zustandsmaschine (CONTRACTS §6): Fenster fehlt ->
// starten, nicht im Vordergrund -> nach vorn holen. Ist das Fenster vorn, aber
// das opencode-Pane nicht fokussiert -> dorthin springen; sonst minimieren.
func Decide(foreground, windowExists, opencodeFocused bool) Action {
	switch {
	case !windowExists:
		return ActionStart
	case !foreground:
		return ActionForeground
	case !opencodeFocused:
		return ActionFocusPane
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
	minimized := exists && win.IsMinimized()
	if minimized {
		// Ein minimiertes Fenster kann sich als "foreground" melden; fuer die
		// Zustandsmaschine zaehlt es als nicht vorn -> wiederherstellen.
		foreground = false
	}
	focused := false
	if d.Herdr != nil {
		focused = opencodeFocused(ctx, d.Herdr, cfg)
	}

	action := Decide(foreground, exists, focused)
	d.logf("toggle: exists=%v foreground=%v minimized=%v opencodeFocused=%v -> %s",
		exists, foreground, minimized, focused, action)
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
	case ActionFocusPane:
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

// opencodeFocused prueft, ob das opencode-Pane gerade den herdr-Fokus hat.
// Primaer ueber das Agent-Flag, mit Fallback ueber die Pane-Liste, da das
// Flag je nach Client/Headless-Kontext abweichen kann.
func opencodeFocused(ctx context.Context, c herdr.Client, cfg *config.Config) bool {
	agents, err := c.AgentList(ctx)
	if err != nil {
		return false
	}
	a, ok := herdr.FindAgent(agents, cfg.AgentName, cfg.AgentKind)
	if !ok {
		return false
	}
	if a.Focused {
		return true
	}
	if a.PaneID == "" {
		return false
	}
	panes, err := c.PaneList(ctx)
	if err != nil {
		return false
	}
	for _, p := range panes {
		if p.ID == a.PaneID {
			return p.Focused
		}
	}
	return false
}

func focusOpencode(ctx context.Context, d Deps, cfg *config.Config) error {
	if d.Herdr == nil {
		return nil
	}
	agents, err := d.Herdr.AgentList(ctx)
	if err != nil {
		// Kein Neustart bei transientem herdr-Fehler: sonst wuerde ein
		// doppelter opencode-Launcher gestartet (und ggf. Control-Sequenzen
		// in die Shell geleakt).
		return fmt.Errorf("herdr agent list: %w", err)
	}
	if a, ok := herdr.FindAgent(agents, cfg.AgentName, cfg.AgentKind); ok {
		// Explizit Workspace -> Tab -> Pane fokussieren: ein Pane-Fokus
		// zieht Tab/Workspace nicht automatisch mit (herdr-jump).
		if a.WorkspaceID != "" {
			_ = d.Herdr.WorkspaceFocus(ctx, a.WorkspaceID)
		}
		if a.TabID != "" {
			_ = d.Herdr.TabFocus(ctx, a.TabID)
		}
		if target := a.Target(); target != "" {
			d.logf("focus: opencode gefunden -> %s", target)
			return d.Herdr.AgentFocus(ctx, target)
		}
		return nil
	}
	d.logf("focus: kein opencode-Agent in %d Agent(en) -> starte neu", len(agents))
	return startOpencode(ctx, d.Herdr, cfg)
}

func startOpencode(ctx context.Context, c herdr.Client, cfg *config.Config) error {
	panes, err := c.PaneList(ctx)
	if err != nil {
		return fmt.Errorf("pane list: %w", err)
	}

	paneID := ""
	// 1) Validierter designierter Pane aus der State-Datei.
	if st, err := LoadPaneState(cfg.PaneStatePath()); err == nil && st != nil && st.PaneID != "" {
		for _, p := range panes {
			if p.ID == st.PaneID {
				paneID = p.ID
				break
			}
		}
	}

	// 2) Frische Session (genau ein agentenloser Pane = Root-Shell): diesen
	//    direkt nutzen statt zu splitten. Ein split legt einen zweiten Pane an
	//    und loest ueber ConPTY eine Resize-Sequenz aus, die in die Shell leakt.
	if paneID == "" && len(panes) == 1 && panes[0].AgentName == "" {
		paneID = panes[0].ID
	}

	// 3) Sonst einen dedizierten Pane erzeugen.
	if paneID == "" {
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
