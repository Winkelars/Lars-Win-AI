package apps

import (
	"context"
	"testing"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

func testWindows() (*window.FakeWindow, *window.FakeWindow, *window.FakeManager) {
	a := window.NewFakeWindow("Terminal", "WindowsTerminal.exe")
	a.Hwnd = 100
	a.Foreground = true
	b := window.NewFakeWindow("Vivaldi", "vivaldi.exe")
	b.Hwnd = 200
	b.WindowClass = "Chrome_WidgetWin_1"
	b.Hidden = true
	return a, b, window.NewFakeManager(a, b)
}

func testSpecs() []Spec {
	return []Spec{
		{ID: "terminal", Process: "WindowsTerminal.exe", Monitor: 2},
		{ID: "vivaldi", Process: "vivaldi.exe", Class: "Chrome_WidgetWin_1", Monitor: 2, Alpha: 235},
	}
}

func TestMatch(t *testing.T) {
	w := window.NewFakeWindow("DeepSeek - Chat", "chrome.exe")
	w.WindowClass = "Chrome_WidgetWin_1"

	if !Match(w, Spec{Process: "chrome.exe", Title: "DeepSeek"}) {
		t.Errorf("Prozess+Titel sollten matchen")
	}
	if Match(w, Spec{Process: "chrome.exe", Title: "Audiobookshelf"}) {
		t.Errorf("falscher Titel darf nicht matchen")
	}
	if Match(w, Spec{Process: "vivaldi.exe"}) {
		t.Errorf("falscher Prozess darf nicht matchen")
	}
	if Match(w, Spec{}) {
		t.Errorf("leerer Spec darf nicht matchen")
	}
	if Match(nil, Spec{Process: "chrome.exe"}) {
		t.Errorf("nil-Fenster darf nicht matchen")
	}
}

func TestTakeAssignsOwnedDialogs(t *testing.T) {
	main := window.NewFakeWindow("Vivaldi", "vivaldi.exe")
	main.Hwnd = 100
	main.WindowClass = "Chrome_WidgetWin_1"
	dialog := window.NewFakeWindow("Datei öffnen", "vivaldi.exe")
	dialog.Hwnd = 101
	dialog.WindowClass = "#32770"
	dialog.OwnerHwnd = 100

	snap, err := Take(window.NewFakeManager(main, dialog), testSpecs())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(snap.Find("vivaldi")); got != 2 {
		t.Fatalf("Dialog sollte dem Slot zugeordnet sein, Fenster=%d", got)
	}
}

func TestToggleShowHidesOtherSlots(t *testing.T) {
	a, b, mgr := testWindows()
	result, err := Toggle(context.Background(), Deps{
		Windows:    mgr,
		Specs:      testSpecs(),
		HideOthers: true,
	}, testSpecs()[1])
	if err != nil {
		t.Fatal(err)
	}
	if result != ResultShow {
		t.Errorf("Result = %v, want %v", result, ResultShow)
	}
	if !b.IsForeground() || b.IsVisible() == false {
		t.Errorf("Vivaldi sollte sichtbar und vorn sein: %+v", b)
	}
	if a.IsVisible() {
		t.Errorf("Terminal sollte versteckt sein")
	}
	if len(b.MoveCalls) != 1 || b.MoveCalls[0] != 2 {
		t.Errorf("MoveAndMaximize(2) erwartet, war %v", b.MoveCalls)
	}
	if b.Alpha() != 235 {
		t.Errorf("Alpha = %d, want 235", b.Alpha())
	}
}

func TestToggleHide(t *testing.T) {
	_, b, mgr := testWindows()
	b.Hidden = false
	b.Foreground = true

	result, err := Toggle(context.Background(), Deps{Windows: mgr, Specs: testSpecs()}, testSpecs()[1])
	if err != nil {
		t.Fatal(err)
	}
	if result != ResultHide {
		t.Errorf("Result = %v, want %v", result, ResultHide)
	}
	if b.IsVisible() || b.IsForeground() {
		t.Errorf("Vivaldi sollte versteckt sein: %+v", b)
	}
}

func TestToggleStartsAndWaits(t *testing.T) {
	mgr := window.NewFakeManager()
	started := [][]string{}
	spec := Spec{ID: "vivaldi", Process: "vivaldi.exe", Monitor: 2, Launch: []string{"vivaldi.exe"}}

	result, err := Toggle(context.Background(), Deps{
		Windows: mgr,
		Specs:   []Spec{spec},
		Timeout: time.Second,
		Sleep:   func(time.Duration) {},
		Start: func(argv []string) error {
			started = append(started, argv)
			w := window.NewFakeWindow("Vivaldi", "vivaldi.exe")
			w.Hwnd = 200
			mgr.Add(w)
			return nil
		},
	}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if result != ResultStart {
		t.Errorf("Result = %v, want %v", result, ResultStart)
	}
	if len(started) != 1 || started[0][0] != "vivaldi.exe" {
		t.Errorf("Startbefehl nicht ausgeführt: %v", started)
	}
	wins := mgr.Windows
	if len(wins) != 1 || !wins[0].IsForeground() {
		t.Errorf("gestartetes Fenster sollte fokussiert sein: %+v", wins)
	}
}

func TestToggleWithoutLaunchFails(t *testing.T) {
	_, err := Toggle(context.Background(), Deps{
		Windows: window.NewFakeManager(),
		Specs:   testSpecs(),
	}, Spec{ID: "leer", Process: "nix.exe", Monitor: 2})
	if err == nil {
		t.Fatal("Fehler erwartet")
	}
}

func TestApplyAlphaTogglesAllSlots(t *testing.T) {
	_, b, mgr := testWindows()
	deps := Deps{Windows: mgr, Specs: testSpecs()}

	if n := ApplyAlpha(deps, true); n != 1 {
		t.Fatalf("ApplyAlpha(on) = %d Fenster, erwartet 1", n)
	}
	if b.Alpha() != 235 {
		t.Errorf("Alpha = %d, erwartet 235", b.Alpha())
	}
	if n := ApplyAlpha(deps, false); n != 1 {
		t.Fatalf("ApplyAlpha(off) = %d Fenster, erwartet 1", n)
	}
	if b.Alpha() != 255 {
		t.Errorf("Alpha = %d, erwartet 255", b.Alpha())
	}
}

func TestShowRespectsAlphaEnabled(t *testing.T) {
	_, b, mgr := testWindows()
	deps := Deps{
		Windows:      mgr,
		Specs:        testSpecs(),
		AlphaEnabled: func() bool { return false },
	}
	if _, err := Toggle(context.Background(), deps, testSpecs()[1]); err != nil {
		t.Fatal(err)
	}
	if b.AlphaCalls != 0 {
		t.Errorf("Alpha darf bei deaktivierter Transparenz nicht gesetzt werden (%d)", b.AlphaCalls)
	}
}

func TestAlphaOffLeavesNativeTransparency(t *testing.T) {
	w := window.NewFakeWindow("YouTube Music", "YouTube Music.exe")
	w.Hwnd = 300
	mgr := window.NewFakeManager(w)
	spec := Spec{ID: "ytmusic", Process: "YouTube Music.exe", Monitor: 2, Alpha: 0}

	if _, err := Toggle(context.Background(), Deps{Windows: mgr, Specs: []Spec{spec}}, spec); err != nil {
		t.Fatal(err)
	}
	if w.AlphaCalls != 0 {
		t.Errorf("Alpha darf bei 0 nicht gesetzt werden (%d Aufrufe)", w.AlphaCalls)
	}
}

func TestHerdrFocusRunsAfterShow(t *testing.T) {
	_, b, mgr := testWindows()
	agent := &fakeAgent{focused: make(chan struct{}, 1)}
	specs := append(testSpecs(), Spec{ID: HerdrID, Kind: HerdrID, Process: "wezterm-gui.exe", Monitor: 2})
	mgr.Add(window.NewFakeWindow("AI-Assistant", "wezterm-gui.exe"))

	if _, err := Toggle(context.Background(), Deps{
		Windows:    mgr,
		Specs:      specs,
		Agent:      agent,
		HideOthers: true,
	}, specs[len(specs)-1]); err != nil {
		t.Fatal(err)
	}
	select {
	case <-agent.focused:
	case <-time.After(2 * time.Second):
		t.Fatal("Agent-Fokus wurde nicht aufgerufen")
	}
	if b.IsVisible() {
		t.Errorf("Vivaldi sollte versteckt sein")
	}
}

type fakeAgent struct{ focused chan struct{} }

func (f *fakeAgent) Focus(context.Context) error {
	f.focused <- struct{}{}
	return nil
}
