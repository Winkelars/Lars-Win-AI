package toggle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/herdr"
	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		foreground   bool
		windowExists bool
		want         Action
	}{
		{foreground: false, windowExists: false, want: ActionStart},
		{foreground: false, windowExists: true, want: ActionForeground},
		{foreground: true, windowExists: false, want: ActionStart},
		{foreground: true, windowExists: true, want: ActionMinimize},
	}
	for _, tc := range cases {
		if got := Decide(tc.foreground, tc.windowExists); got != tc.want {
			t.Errorf("Decide(%v,%v) = %v, want %v",
				tc.foreground, tc.windowExists, got, tc.want)
		}
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.PaneStateFile = filepath.Join(t.TempDir(), "pane-state.json")
	cfg.StartupTimeoutMS = 500
	return cfg
}

func swapStartProcess(fn func(*exec.Cmd) error) func() {
	old := startProcess
	startProcess = fn
	return func() { startProcess = old }
}

func TestRunActionForeground(t *testing.T) {
	cfg := testConfig(t)
	w := window.NewFakeWindow("AI-Assistant", "alacritty.exe")
	w.Foreground = false
	w.Minimized = true
	mgr := window.NewFakeManager(w)
	stub := &herdr.StubClient{
		Agents: []herdr.Agent{{Name: "opencode", Kind: "opencode", Focused: true}},
	}
	act, err := Run(context.Background(), Deps{Windows: mgr, Herdr: stub, CFG: cfg}, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if act != ActionForeground {
		t.Fatalf("action = %v", act)
	}
	if w.RestoreCalls != 1 || len(w.MoveCalls) != 1 || w.MoveCalls[0] != 2 || w.FocusCalls != 1 {
		t.Errorf("Fensteraktionen falsch: %+v", w)
	}
	found := false
	for _, c := range stub.Calls() {
		if c == "agent focus opencode" {
			found = true
		}
	}
	if !found {
		t.Errorf("AgentFocus nicht aufgerufen: %v", stub.Calls())
	}
}

func TestRunActionMinimize(t *testing.T) {
	cfg := testConfig(t)
	w := window.NewFakeWindow("AI-Assistant", "alacritty.exe")
	w.Foreground = true
	mgr := window.NewFakeManager(w)
	stub := &herdr.StubClient{
		Agents: []herdr.Agent{{Name: "opencode", Kind: "opencode", Focused: true}},
	}
	act, err := Run(context.Background(), Deps{Windows: mgr, Herdr: stub, CFG: cfg}, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if act != ActionMinimize {
		t.Fatalf("action = %v", act)
	}
	if w.MinimizeCalls != 1 {
		t.Errorf("Minimize nicht aufgerufen: %+v", w)
	}
	for _, c := range stub.Calls() {
		if c == "agent focus opencode" {
			t.Errorf("AgentFocus sollte nicht aufgerufen werden: %v", stub.Calls())
		}
	}
}

func TestRunActionStart(t *testing.T) {
	cfg := testConfig(t)
	frame := window.NewFakeWindow("AI-Assistant", "alacritty.exe")

	lookups := 0
	mgr := &window.FakeManager{
		FindTitleFn: func(string) (window.Window, bool) {
			lookups++
			if lookups >= 2 {
				return frame, true
			}
			return nil, false
		},
		FindProcessFn: func(string) (window.Window, bool) { return nil, false },
	}
	stub := &herdr.StubClient{
		Panes:   []herdr.Pane{{ID: "pane-1", AgentName: ""}},
		SplitID: "pane-9",
	}

	restore := swapStartProcess(func(_ *exec.Cmd) error { return nil })
	defer restore()

	act, err := Run(context.Background(), Deps{Windows: mgr, Herdr: stub, CFG: cfg}, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if act != ActionStart {
		t.Fatalf("action = %v", act)
	}
	if len(frame.MoveCalls) != 1 || frame.MoveCalls[0] != 2 || frame.FocusCalls != 1 {
		t.Errorf("Fensteraktionen falsch: %+v", frame)
	}
	calls := stub.Calls()
	wantSplit := false
	wantStart := false
	for _, c := range calls {
		if c == "pane split pane-1 right" {
			wantSplit = true
		}
		if c == "agent start opencode kind=opencode pane=pane-9" {
			wantStart = true
		}
	}
	if !wantSplit || !wantStart {
		t.Errorf("opencode-Pane nicht gestartet: %v", calls)
	}

	st, err := LoadPaneState(cfg.PaneStatePath())
	if err != nil {
		t.Fatalf("LoadPaneState: %v", err)
	}
	if st == nil || st.PaneID != "pane-9" {
		t.Errorf("PaneState = %+v", st)
	}
}

func TestPaneStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "pane.json")
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
