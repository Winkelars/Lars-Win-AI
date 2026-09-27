package hotkey

import "testing"

func defaultCfg() Config {
	return Config{ScanCode: 41, AltOnly: true, ExcludeAltGr: true}
}

func TestMatchesHotkey(t *testing.T) {
	cfg := defaultCfg()
	cases := []struct {
		name string
		e    Event
		want bool
	}{
		{"Alt+ScanCode keydown", Event{ScanCode: 41, KeyDown: true, Alt: true}, true},
		{"Alt links oder rechts egal", Event{ScanCode: 41, KeyDown: true, Alt: true}, true},
		{"keyup zaehlt nicht", Event{ScanCode: 41, KeyDown: false, Alt: true}, false},
		{"ohne Alt", Event{ScanCode: 41, KeyDown: true}, false},
		{"AltGr (Ctrl+Alt) ausgeschlossen", Event{ScanCode: 41, KeyDown: true, Alt: true, Ctrl: true}, false},
		{"mit Shift ausgeschlossen", Event{ScanCode: 41, KeyDown: true, Alt: true, Shift: true}, false},
		{"mit Win ausgeschlossen", Event{ScanCode: 41, KeyDown: true, Alt: true, Win: true}, false},
		{"falscher ScanCode", Event{ScanCode: 42, KeyDown: true, Alt: true}, false},
		{"injiziert ignoriert", Event{ScanCode: 41, KeyDown: true, Alt: true, Injected: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Matches(tc.e, cfg); got != tc.want {
				t.Errorf("Matches(%+v) = %v, want %v", tc.e, got, tc.want)
			}
		})
	}
}

func TestMatchesAltOnlyFalseAllowsExtraModifiers(t *testing.T) {
	cfg := Config{ScanCode: 41, AltOnly: false, ExcludeAltGr: false}
	if !Matches(Event{ScanCode: 41, KeyDown: true, Alt: true, Shift: true}, cfg) {
		t.Errorf("mit AltOnly=false sollte Shift erlaubt sein")
	}
	if !Matches(Event{ScanCode: 41, KeyDown: true, Alt: true, Ctrl: true}, cfg) {
		t.Errorf("mit ExcludeAltGr=false sollte Ctrl erlaubt sein")
	}
}

func TestDetectorOnlyFiresOnEdge(t *testing.T) {
	d := NewDetector(defaultCfg())
	down := Event{ScanCode: 41, KeyDown: true, Alt: true}
	up := Event{ScanCode: 41, KeyDown: false, Alt: true}

	if !d.Feed(down) {
		t.Fatalf("erster Druck muss auslösen")
	}
	if d.Feed(down) {
		t.Fatalf("gehaltener Hotkey darf nicht erneut auslösen")
	}
	if d.Feed(up) {
		t.Fatalf("KeyUp darf nicht auslösen")
	}
	if !d.Feed(down) {
		t.Fatalf("nach Loslassen muss erneut auslösen")
	}
}
