package window

import "testing"

func TestTitleMatches(t *testing.T) {
	if !TitleMatches("AI-Assistant", "ai-assistant") {
		t.Errorf("case-insensitive Match erwartet")
	}
	if !TitleMatches("prefix AI-Assistant suffix", "AI-Assistant") {
		t.Errorf("Contains-Match erwartet")
	}
	if TitleMatches("etwas anderes", "AI-Assistant") {
		t.Errorf("kein Match erwartet")
	}
	if TitleMatches("AI-Assistant", "") {
		t.Errorf("leerer Suchtitel darf nicht matchen")
	}
}

func TestProcessMatches(t *testing.T) {
	if !ProcessMatches("wezterm-gui.exe", "wezterm-gui.exe") {
		t.Errorf("identische Namen erwartet")
	}
	if !ProcessMatches("WezTerm-GUI", "wezterm-gui.exe") {
		t.Errorf(".exe-Endung soll ignoriert werden")
	}
	if ProcessMatches("notepad.exe", "wezterm-gui.exe") {
		t.Errorf("kein Match erwartet")
	}
}

func TestFindFallsBackToProcess(t *testing.T) {
	w := NewFakeWindow("irgendein Titel", "wezterm-gui.exe")
	m := NewFakeManager(w)
	got, ok := Find(m, "AI-Assistant", "wezterm-gui.exe")
	if !ok || got != Window(w) {
		t.Fatalf("Prozess-Fallback fehlgeschlagen: %v %v", got, ok)
	}
	if _, ok := Find(m, "AI-Assistant", "notepad.exe"); ok {
		t.Fatalf("kein Fenster erwartet")
	}
}

func TestFakeWindowActions(t *testing.T) {
	w := NewFakeWindow("AI-Assistant", "wezterm-gui.exe")
	if err := w.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := w.Focus(); err != nil {
		t.Fatal(err)
	}
	if !w.IsForeground() {
		t.Errorf("nach Focus sollte Vordergrund true sein")
	}
	if err := w.MoveAndMaximize(2); err != nil {
		t.Fatal(err)
	}
	if w.Monitor != 2 || len(w.MoveCalls) != 1 {
		t.Errorf("MoveAndMaximize nicht aufgezeichnet: %+v", w)
	}
	if err := w.Minimize(); err != nil {
		t.Fatal(err)
	}
	if !w.Minimized || w.IsForeground() {
		t.Errorf("Minimize-Zustand falsch: %+v", w)
	}
}
