package hotkey

import "testing"

func testBinding() Binding {
	return Binding{Action: "terminal", ScanCode: 2, AltOnly: true, ExcludeAltGr: true}
}

func TestSetMatch(t *testing.T) {
	set := NewSet([]Binding{testBinding()})

	ok := Event{ScanCode: 2, KeyDown: true, Alt: true}
	if b, matched := set.Match(ok); !matched || b.Action != "terminal" {
		t.Fatalf("Alt+ScanCode 2 sollte matchen: %+v %v", b, matched)
	}

	cases := []struct {
		name  string
		event Event
	}{
		{"ohne Alt", Event{ScanCode: 2, KeyDown: true}},
		{"mit Ctrl", Event{ScanCode: 2, KeyDown: true, Alt: true, Ctrl: true}},
		{"mit Shift", Event{ScanCode: 2, KeyDown: true, Alt: true, Shift: true}},
		{"mit Win", Event{ScanCode: 2, KeyDown: true, Alt: true, Win: true}},
		{"Keyup", Event{ScanCode: 2, KeyDown: false, Alt: true}},
		{"injiziert", Event{ScanCode: 2, KeyDown: true, Alt: true, Injected: true}},
		{"andere Taste", Event{ScanCode: 3, KeyDown: true, Alt: true}},
	}
	for _, tc := range cases {
		if _, matched := set.Match(tc.event); matched {
			t.Errorf("%s: kein Match erwartet", tc.name)
		}
	}
}

func TestNewSetDropsEmptyScanCode(t *testing.T) {
	set := NewSet([]Binding{{Action: "kaputt"}})
	if len(set.Bindings()) != 0 {
		t.Errorf("ScanCode 0 muss verworfen werden: %+v", set.Bindings())
	}
}

func TestRouterRisingEdge(t *testing.T) {
	router := NewRouter(NewSet([]Binding{testBinding()}))
	down := Event{ScanCode: 2, KeyDown: true, Alt: true}
	up := Event{ScanCode: 2, KeyDown: false, Alt: true}

	if _, ok := router.Feed(down); !ok {
		t.Fatal("erster Druck sollte auslösen")
	}
	if _, ok := router.Feed(down); ok {
		t.Fatal("gehaltene Taste darf nur einmal auslösen")
	}
	router.Feed(up)
	if _, ok := router.Feed(down); !ok {
		t.Fatal("nach Keyup sollte erneut ausgelöst werden")
	}
}

func TestRouterIgnoresUnboundKeys(t *testing.T) {
	router := NewRouter(NewSet([]Binding{testBinding()}))
	if _, ok := router.Feed(Event{ScanCode: 30, KeyDown: true, Alt: true}); ok {
		t.Fatal("ungebundene Taste darf nicht auslösen")
	}
}
