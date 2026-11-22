package hotkey

import "strings"

// Binding verbindet eine Taste mit einer Aktion.
type Binding struct {
	// Action ist der stabile Bezeichner, den der Daemon ausführt
	// ("herdr" oder eine App-ID).
	Action string
	// ScanCode ist der hardwarenahe Scancode (layoutunabhängig).
	ScanCode     uint32
	AltOnly      bool
	ExcludeAltGr bool
}

// Set ist eine geordnete Menge von Bindings. Die Reihenfolge entscheidet bei
// doppelten Scancodes: das erste Binding gewinnt.
type Set struct {
	bindings []Binding
}

// NewSet kopiert die Bindings in ein Set.
func NewSet(bindings []Binding) Set {
	out := make([]Binding, 0, len(bindings))
	for _, b := range bindings {
		if b.ScanCode == 0 {
			continue
		}
		out = append(out, b)
	}
	return Set{bindings: out}
}

// Bindings liefert eine Kopie der Bindings.
func (s Set) Bindings() []Binding {
	out := make([]Binding, len(s.bindings))
	copy(out, s.bindings)
	return out
}

// Match prüft, ob ein Keydown-Event zu einem Binding passt.
func (s Set) Match(e Event) (Binding, bool) {
	if !e.KeyDown || e.Injected {
		return Binding{}, false
	}
	for _, b := range s.bindings {
		if e.ScanCode != b.ScanCode || !e.Alt {
			continue
		}
		if b.ExcludeAltGr && e.Ctrl {
			continue
		}
		if b.AltOnly && (e.Ctrl || e.Shift || e.Win) {
			continue
		}
		return b, true
	}
	return Binding{}, false
}

// Matches meldet nur, ob ein Event zu irgendeinem Binding passt.
func (s Set) Matches(e Event) bool {
	_, ok := s.Match(e)
	return ok
}

// ScanCodes liefert alle gebundenen Scancodes (für Diagnose und Tests).
func (s Set) ScanCodes() []uint32 {
	out := make([]uint32, 0, len(s.bindings))
	for _, b := range s.bindings {
		out = append(out, b.ScanCode)
	}
	return out
}

// Router erkennt die steigende Flanke eines Hotkey-Drucks. Er ist
// zustandsbehaftet, damit ein gehaltener Hotkey nur einmal auslöst.
type Router struct {
	set  Set
	down map[uint32]bool
}

// NewRouter erstellt einen Router für das Set.
func NewRouter(s Set) *Router {
	return &Router{set: s, down: make(map[uint32]bool)}
}

// Feed verarbeitet ein Event und meldet das getroffene Binding beim ersten
// Keydown. Keyups setzen den Zustand zurück, lösen aber nichts aus.
func (r *Router) Feed(e Event) (Binding, bool) {
	if r == nil {
		return Binding{}, false
	}
	if !e.KeyDown {
		delete(r.down, e.ScanCode)
		return Binding{}, false
	}
	b, ok := r.set.Match(e)
	if !ok {
		return Binding{}, false
	}
	if r.down[b.ScanCode] {
		return Binding{}, false
	}
	r.down[b.ScanCode] = true
	return b, true
}

// ActionLabel liefert eine lesbare Bezeichnung für Log und Status.
func ActionLabel(b Binding) string {
	if strings.TrimSpace(b.Action) != "" {
		return b.Action
	}
	return "unbenannt"
}
