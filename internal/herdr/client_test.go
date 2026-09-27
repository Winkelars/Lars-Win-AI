package herdr

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const agentListJSON = `{"result":{"agents":[
  {"agent":"opencode","name":"oc-main","agent_status":"running","focused":true,"pane_id":"pane-1","tab_id":"tab-1","workspace_id":"ws-1"},
  {"agent":"shell","name":"sh","agent_status":"idle","focused":false,"pane_id":"pane-2","tab_id":"tab-1","workspace_id":"ws-1"}
]}}`

const agentListJSONWithKind = `{"result":{"agents":[
  {"agent":"other","kind":"opencode","agent_status":"running","focused":false,"pane_id":"pane-7","tab_id":"tab-2","workspace_id":"ws-1"}
]}}`

const agentListJSONUnnamed = `{"result":{"agents":[
  {"agent":"opencode","agent_status":"running","focused":true,"pane_id":"pane-5","tab_id":"tab-1","workspace_id":"ws-1"}
]}}`

const paneListJSON = `{"result":{"panes":[
  {"pane_id":"pane-1","tab_id":"tab-1","workspace_id":"ws-1","cwd":"C:\\code","agent":"opencode"},
  {"pane_id":"pane-2","tab_id":"tab-1","workspace_id":"ws-1","cwd":"C:\\code","agent":""}
]}}`

const paneSplitJSON = `{"result":{"pane":{"pane_id":"pane-9","tab_id":"tab-1","workspace_id":"ws-1"}}}`

func TestParseAgentList(t *testing.T) {
	agents, err := ParseAgentList([]byte(agentListJSON))
	if err != nil {
		t.Fatalf("ParseAgentList: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("len = %d", len(agents))
	}
	a := agents[0]
	if a.Label != "opencode" || a.Name != "oc-main" || a.Status != "running" || !a.Focused {
		t.Errorf("Agent = %+v", a)
	}
	if a.PaneID != "pane-1" || a.TabID != "tab-1" || a.WorkspaceID != "ws-1" {
		t.Errorf("IDs = %+v", a)
	}
}

func TestParseAgentListArrayFallback(t *testing.T) {
	agents, err := ParseAgentList([]byte(`[{"agent":"x","focused":true}]`))
	if err != nil {
		t.Fatalf("ParseAgentList: %v", err)
	}
	if len(agents) != 1 || agents[0].Label != "x" || !agents[0].Focused {
		t.Fatalf("agents = %+v", agents)
	}
}

func TestParsePaneList(t *testing.T) {
	panes, err := ParsePaneList([]byte(paneListJSON))
	if err != nil {
		t.Fatalf("ParsePaneList: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("len = %d", len(panes))
	}
	if panes[0].ID != "pane-1" || panes[0].AgentName != "opencode" || panes[0].CWD != `C:\code` {
		t.Errorf("Pane = %+v", panes[0])
	}
}

func TestParsePaneSplit(t *testing.T) {
	id, err := ParsePaneSplit([]byte(paneSplitJSON))
	if err != nil {
		t.Fatalf("ParsePaneSplit: %v", err)
	}
	if id != "pane-9" {
		t.Errorf("id = %q", id)
	}
	if _, err := ParsePaneSplit([]byte(`{"result":{}}`)); err == nil {
		t.Errorf("erwartet Fehler bei fehlender pane_id")
	}
}

func TestFindAgent(t *testing.T) {
	agents, _ := ParseAgentList([]byte(agentListJSON))
	if a, ok := FindAgent(agents, "sh", ""); !ok || a.Name != "sh" {
		t.Errorf("Name-Match: %+v ok=%v", a, ok)
	}
	if a, ok := FindAgent(agents, "missing", ""); ok {
		t.Errorf("unerwarteter Match: %+v", a)
	}

	// Agent ohne eindeutigen Namen: Match über Label, Fokus-Ziel = Pane-ID.
	unnamed, _ := ParseAgentList([]byte(agentListJSONUnnamed))
	if a, ok := FindAgent(unnamed, "opencode", "opencode"); !ok || a.Target() != "pane-5" {
		t.Errorf("Label-Match: %+v ok=%v target=%q", a, ok, a.Target())
	}

	kindAgents, _ := ParseAgentList([]byte(agentListJSONWithKind))
	if a, ok := FindAgent(kindAgents, "missing", "opencode"); !ok || a.Kind != "opencode" {
		t.Errorf("Kind-Match: %+v ok=%v", a, ok)
	}
}

func TestExecClientBuildsArgs(t *testing.T) {
	var got []string
	c := New("herdr", 0, 0)
	c.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "herdr" {
			t.Fatalf("name = %q", name)
		}
		got = append([]string{}, args...)
		return []byte(agentListJSON), nil
	}
	if _, err := c.AgentList(context.Background()); err != nil {
		t.Fatalf("AgentList: %v", err)
	}
	if strings.Join(got, " ") != "agent list" {
		t.Errorf("args = %v", got)
	}

	if err := c.AgentFocus(context.Background(), "opencode"); err != nil {
		t.Fatalf("AgentFocus: %v", err)
	}
	if strings.Join(got, " ") != "agent focus opencode" {
		t.Errorf("args = %v", got)
	}

	c.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		got = append([]string{}, args...)
		return []byte(paneSplitJSON), nil
	}
	if _, err := c.PaneSplit(context.Background(), "pane-1", "right"); err != nil {
		t.Fatalf("PaneSplit: %v", err)
	}
	if strings.Join(got, " ") != "pane split pane-1 --direction right --no-focus" {
		t.Errorf("args = %v", got)
	}

	if err := c.AgentStart(context.Background(), "opencode", "opencode", "pane-9", []string{"--auto"}); err != nil {
		t.Fatalf("AgentStart: %v", err)
	}
	if strings.Join(got, " ") != "agent start opencode --kind opencode --pane pane-9 -- --auto" {
		t.Errorf("args = %v", got)
	}
}

func TestExecClientRetriesOnServerError(t *testing.T) {
	attempts := 0
	c := New("herdr", 3, time.Millisecond)
	c.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("herdr: dial unix socket: connect: connection refused")
		}
		return []byte(agentListJSON), nil
	}
	if _, err := c.AgentList(context.Background()); err != nil {
		t.Fatalf("AgentList: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, erwartet 3", attempts)
	}
}

func TestExecClientDoesNotRetryOtherErrors(t *testing.T) {
	attempts := 0
	c := New("herdr", 5, time.Millisecond)
	c.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts++
		return nil, errors.New("herdr agent list: exit status 1")
	}
	if _, err := c.AgentList(context.Background()); err == nil {
		t.Fatalf("erwartet Fehler")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, erwartet 1", attempts)
	}
}

func TestStubClient(t *testing.T) {
	s := &StubClient{
		Agents:  []Agent{{Name: "opencode", Kind: "opencode", Focused: false}},
		Panes:   []Pane{{ID: "pane-1"}},
		SplitID: "pane-9",
	}
	agents, err := s.AgentList(context.Background())
	if err != nil || len(agents) != 1 {
		t.Fatalf("AgentList: %v %v", agents, err)
	}
	if err := s.AgentFocus(context.Background(), "opencode"); err != nil {
		t.Fatalf("AgentFocus: %v", err)
	}
	if !s.Agents[0].Focused {
		t.Errorf("Fokus nicht gesetzt")
	}
	id, err := s.PaneSplit(context.Background(), "pane-1", "right")
	if err != nil || id != "pane-9" {
		t.Fatalf("PaneSplit: %q %v", id, err)
	}
}
