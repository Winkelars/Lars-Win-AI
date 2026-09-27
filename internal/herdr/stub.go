package herdr

import (
	"context"
	"errors"
	"sync"
)

// StubClient ist eine konfigurierbare In-Memory-Implementierung von Client
// für Tests und den --no-hook-Betrieb.
type StubClient struct {
	Agents  []Agent
	Panes   []Pane
	SplitID string

	AgentListFn      func(ctx context.Context) ([]Agent, error)
	AgentFocusFn     func(ctx context.Context, name string) error
	AgentStartFn     func(ctx context.Context, name, kind, paneID string, args []string) error
	PaneListFn       func(ctx context.Context) ([]Pane, error)
	PaneSplitFn      func(ctx context.Context, paneID, direction string) (string, error)
	WorkspaceFocusFn func(ctx context.Context, workspaceID string) error
	TabFocusFn       func(ctx context.Context, tabID string) error

	mu    sync.Mutex
	calls []string
}

// Calls liefert die aufgezeichneten Aufrufe.
func (s *StubClient) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *StubClient) record(call string) {
	s.mu.Lock()
	s.calls = append(s.calls, call)
	s.mu.Unlock()
}

// AgentList liefert die konfigurierten Agenten.
func (s *StubClient) AgentList(ctx context.Context) ([]Agent, error) {
	s.record("agent list")
	if s.AgentListFn != nil {
		return s.AgentListFn(ctx)
	}
	out := make([]Agent, len(s.Agents))
	copy(out, s.Agents)
	return out, nil
}

// AgentFocus zeichnet den Fokus auf.
func (s *StubClient) AgentFocus(ctx context.Context, name string) error {
	s.record("agent focus " + name)
	if s.AgentFocusFn != nil {
		return s.AgentFocusFn(ctx, name)
	}
	for i := range s.Agents {
		if s.Agents[i].Name == name {
			s.Agents[i].Focused = true
		} else {
			s.Agents[i].Focused = false
		}
	}
	return nil
}

// AgentStart zeichnet den Start auf.
func (s *StubClient) AgentStart(ctx context.Context, name, kind, paneID string, args []string) error {
	s.record("agent start " + name + " kind=" + kind + " pane=" + paneID)
	if s.AgentStartFn != nil {
		return s.AgentStartFn(ctx, name, kind, paneID, args)
	}
	s.Agents = append(s.Agents, Agent{Name: name, Kind: kind, PaneID: paneID, Focused: true})
	return nil
}

// PaneList liefert die konfigurierten Panes.
func (s *StubClient) PaneList(ctx context.Context) ([]Pane, error) {
	s.record("pane list")
	if s.PaneListFn != nil {
		return s.PaneListFn(ctx)
	}
	out := make([]Pane, len(s.Panes))
	copy(out, s.Panes)
	return out, nil
}

// PaneSplit liefert die konfigurierte neue Pane-ID.
func (s *StubClient) PaneSplit(ctx context.Context, paneID, direction string) (string, error) {
	s.record("pane split " + paneID + " " + direction)
	if s.PaneSplitFn != nil {
		return s.PaneSplitFn(ctx, paneID, direction)
	}
	if s.SplitID == "" {
		return "", errors.New("stub: keine SplitID gesetzt")
	}
	return s.SplitID, nil
}

var _ Client = (*StubClient)(nil)
var _ Client = (*ExecClient)(nil)

// WorkspaceFocus zeichnet den Fokus auf.
func (s *StubClient) WorkspaceFocus(ctx context.Context, workspaceID string) error {
	s.record("workspace focus " + workspaceID)
	if s.WorkspaceFocusFn != nil {
		return s.WorkspaceFocusFn(ctx, workspaceID)
	}
	return nil
}

// TabFocus zeichnet den Fokus auf.
func (s *StubClient) TabFocus(ctx context.Context, tabID string) error {
	s.record("tab focus " + tabID)
	if s.TabFocusFn != nil {
		return s.TabFocusFn(ctx, tabID)
	}
	return nil
}
