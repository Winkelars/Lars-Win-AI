// Package toggle enthält die Herdr-/opencode-Integration: Pane-Fokus,
// Agent-Start und der designierte Pane-State (CONTRACTS §6). Die eigentliche
// Slot-Zustandsmaschine liegt in internal/apps.
package toggle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/herdr"
)

// Deps bündelt die Abhängigkeiten der Agent-Integration.
type Deps struct {
	Herdr herdr.Client
	CFG   *config.Config
	// Logf ist optional und dient der Diagnose (z. B. log.Debugf).
	Logf func(format string, args ...interface{})
}

func (d Deps) logf(format string, args ...interface{}) {
	if d.Logf != nil {
		d.Logf(format, args...)
	}
}

// FocusOpencode springt auf das opencode-Pane. Fehlt der Agent, wird er in
// einem freien bzw. neuen Pane gestartet.
func FocusOpencode(ctx context.Context, d Deps, cfg *config.Config) error {
	if d.Herdr == nil {
		return nil
	}
	agents, err := agentListRetry(ctx, d.Herdr, 5, 200*time.Millisecond)
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
	return StartOpencode(ctx, d.Herdr, cfg)
}

// StartOpencode startet den Agenten in einem geeigneten Pane.
func StartOpencode(ctx context.Context, c herdr.Client, cfg *config.Config) error {
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
		if strings.Contains(err.Error(), "agent_pane_busy") {
			// Der Pane beherbergt bereits einen Agenten (herdr agent list war
			// transient leer) - kein erneuter Launcher, kein Leak.
			return nil
		}
		return fmt.Errorf("agent start: %w", err)
	}
	_ = SavePaneState(cfg.PaneStatePath(), &PaneState{
		PaneID:    paneID,
		UpdatedAt: time.Now().UTC(),
	})
	return nil
}

// agentListRetry liest die Agentenliste und wiederholt bei leerer/fehlerhafter
// Antwort, weil herdrs `agent list` bei Headless-Aufrufen kurzzeitig leer sein
// kann (z. B. direkt nach Verstecken/Wiederherstellen).
func agentListRetry(ctx context.Context, c herdr.Client, attempts int, delay time.Duration) ([]herdr.Agent, error) {
	var agents []herdr.Agent
	var err error
	for i := 0; i < attempts; i++ {
		agents, err = c.AgentList(ctx)
		if err == nil && len(agents) > 0 {
			return agents, nil
		}
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return agents, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return agents, err
}
