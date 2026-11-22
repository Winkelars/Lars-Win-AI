package toggle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/herdr"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.PaneStateFile = filepath.Join(t.TempDir(), "pane-state.json")
	cfg.StartupTimeoutMS = 500
	return cfg
}

func TestFocusOpencodeSequence(t *testing.T) {
	cfg := testConfig(t)
	stub := &herdr.StubClient{
		Agents: []herdr.Agent{{
			Name:        "opencode",
			Kind:        "opencode",
			PaneID:      "w1:p1",
			TabID:       "w1:t1",
			WorkspaceID: "w1",
		}},
	}
	if err := FocusOpencode(context.Background(), Deps{Herdr: stub, CFG: cfg}, cfg); err != nil {
		t.Fatalf("FocusOpencode: %v", err)
	}
	got := strings.Join(stub.Calls(), " | ")
	last := -1
	for _, want := range []string{"workspace focus w1", "tab focus w1:t1", "agent focus opencode"} {
		i := strings.Index(got, want)
		if i < 0 {
			t.Fatalf("fehlt %q in: %s", want, got)
		}
		if i < last {
			t.Fatalf("falsche Reihenfolge (%q): %s", want, got)
		}
		last = i
	}
}

func TestFocusOpencodeStartsMissingAgent(t *testing.T) {
	cfg := testConfig(t)
	stub := &herdr.StubClient{
		Panes:   []herdr.Pane{{ID: "pane-1", AgentName: ""}},
		SplitID: "pane-9",
	}
	if err := FocusOpencode(context.Background(), Deps{Herdr: stub, CFG: cfg}, cfg); err != nil {
		t.Fatalf("FocusOpencode: %v", err)
	}
	calls := strings.Join(stub.Calls(), " | ")
	if strings.Contains(calls, "pane split") {
		t.Errorf("unerwarteter pane split: %s", calls)
	}
	if !strings.Contains(calls, "agent start opencode kind=opencode pane=pane-1") {
		t.Errorf("agent start fehlt: %s", calls)
	}
	st, err := LoadPaneState(cfg.PaneStatePath())
	if err != nil {
		t.Fatalf("LoadPaneState: %v", err)
	}
	if st == nil || st.PaneID != "pane-1" {
		t.Errorf("PaneState = %+v", st)
	}
}

func TestStartOpencodeSplitsWhenPanesExist(t *testing.T) {
	cfg := testConfig(t)
	stub := &herdr.StubClient{
		Panes:   []herdr.Pane{{ID: "pane-1", AgentName: ""}, {ID: "pane-2", AgentName: ""}},
		SplitID: "pane-9",
	}
	if err := StartOpencode(context.Background(), stub, cfg); err != nil {
		t.Fatalf("StartOpencode: %v", err)
	}
	calls := strings.Join(stub.Calls(), " | ")
	if !strings.Contains(calls, "pane split pane-1 right") {
		t.Errorf("erwarteter split fehlt: %s", calls)
	}
	if !strings.Contains(calls, "agent start opencode kind=opencode pane=pane-9") {
		t.Errorf("agent start fehlt: %s", calls)
	}
}

func TestPaneStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "pane.json")
	if st, err := LoadPaneState(path); err != nil || st != nil {
		t.Fatalf("fehlende Datei: %+v %v", st, err)
	}
	if err := SavePaneState(path, &PaneState{PaneID: "pane-3", TabID: "tab-1"}); err != nil {
		t.Fatalf("SavePaneState: %v", err)
	}
	st, err := LoadPaneState(path)
	if err != nil || st == nil || st.PaneID != "pane-3" || st.TabID != "tab-1" {
		t.Fatalf("RoundTrip = %+v %v", st, err)
	}
}

func TestPaneStateInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPaneState(path); err == nil {
		t.Fatalf("erwartet Fehler bei ungültigem JSON")
	}
}
