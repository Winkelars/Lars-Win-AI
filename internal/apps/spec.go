// Package apps verwaltet Fenster-Slots: jeder Slot ist eine App (oder der
// Herdr-Slot), die entweder sichtbar und fokussiert oder komplett versteckt
// ist. "Aktiv" wird ueber IsForeground bestimmt, nicht ueber einen Cache —
// damit ist der Zustand selbstheilend.
package apps

import (
	"strings"

	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

// HerdrID ist die reserved ID des Herdr-Slots.
const HerdrID = "herdr"

// AlphaOff bedeutet: keine kuenstliche Transparenz setzen.
const AlphaOff = 255

// Spec beschreibt einen Slot (Fenster-Matcher, Startbefehl, Monitor, Alpha).
type Spec struct {
	ID       string
	Kind     string
	ScanCode uint32
	Launch   []string
	Process  string
	Class    string
	Title    string
	Monitor  int
	Alpha    int
}

// IsValid meldet, ob mindestens ein Matcher gesetzt ist.
func (s Spec) IsValid() bool {
	return strings.TrimSpace(s.Process) != "" ||
		strings.TrimSpace(s.Class) != "" ||
		strings.TrimSpace(s.Title) != ""
}

// IsHerdr meldet, ob der Slot die Herdr-Konsole ist.
func (s Spec) IsHerdr() bool { return s.Kind == HerdrID }

// Match prueft ein Fenster gegen den Slot. Teure Pruefungen (Prozessname)
// stehen bewusst am Ende.
func Match(w window.Window, s Spec) bool {
	if w == nil || !s.IsValid() {
		return false
	}
	if strings.TrimSpace(s.Title) != "" && !window.TitleMatches(w.Title(), s.Title) {
		return false
	}
	if strings.TrimSpace(s.Class) != "" && !window.ClassMatches(w.ClassName(), s.Class) {
		return false
	}
	if strings.TrimSpace(s.Process) != "" && !window.ProcessMatches(w.ProcessName(), s.Process) {
		return false
	}
	return true
}

// entry cached die teuren Fenster-Abfragen waehrend eines Snapshots.
type entry struct {
	w     window.Window
	title string
	class string
	pid   uint32
	name  string
	named bool
}

func newEntry(w window.Window) *entry {
	return &entry{w: w, title: w.Title(), class: w.ClassName(), pid: w.PID()}
}

// processName fragt den Prozessnamen hoechstens einmal pro Fenster ab.
func (e *entry) processName() string {
	if !e.named {
		e.name = e.w.ProcessName()
		e.named = true
	}
	return e.name
}

func (e *entry) match(s Spec) bool {
	if !s.IsValid() {
		return false
	}
	if strings.TrimSpace(s.Title) != "" && !window.TitleMatches(e.title, s.Title) {
		return false
	}
	if strings.TrimSpace(s.Class) != "" && !window.ClassMatches(e.class, s.Class) {
		return false
	}
	if strings.TrimSpace(s.Process) != "" && !window.ProcessMatches(e.processName(), s.Process) {
		return false
	}
	return true
}

// Snapshot ist eine Momentaufnahme aller Fenster, gruppiert nach Slot.
type Snapshot struct {
	ByID map[string][]window.Window
	// Direct enthaelt nur Fenster, die direkt am Matcher des Slots haengen —
	// ohne zugeordnete Dialoge. Fuer Transparenz ist das die richtige Menge:
	// Popups und Menues bleiben deckend.
	Direct map[string][]window.Window
	All    []window.Window
}

// Take enumeriert alle Top-Level-Fenster (auch versteckte) und ordnet sie den
// Slots zu. Fenster ohne eigenen Matcher werden ueber ihren Owner dem Slot
// ihres Aufrufers zugeschlagen (Dialoge).
func Take(m window.Manager, specs []Spec) (Snapshot, error) {
	if m == nil {
		return Snapshot{}, nil
	}
	all, err := m.Enumerate()
	if err != nil {
		return Snapshot{}, err
	}
	entries := make([]*entry, 0, len(all))
	for _, w := range all {
		entries = append(entries, newEntry(w))
	}

	snap := Snapshot{
		ByID:   make(map[string][]window.Window, len(specs)),
		Direct: make(map[string][]window.Window, len(specs)),
		All:    all,
	}
	assigned := make(map[uintptr]string, len(all))
	for _, s := range specs {
		for _, e := range entries {
			if _, taken := assigned[e.w.Handle()]; taken {
				continue
			}
			if e.match(s) {
				snap.ByID[s.ID] = append(snap.ByID[s.ID], e.w)
				snap.Direct[s.ID] = append(snap.Direct[s.ID], e.w)
				assigned[e.w.Handle()] = s.ID
			}
		}
	}
	for _, e := range entries {
		owner := e.w.Owner()
		if owner == 0 {
			continue
		}
		if _, taken := assigned[e.w.Handle()]; taken {
			continue
		}
		if id, ok := assigned[owner]; ok {
			snap.ByID[id] = append(snap.ByID[id], e.w)
			assigned[e.w.Handle()] = id
		}
	}
	return snap, nil
}

// MainWindow waehlt das wahrscheinlichste Hauptfenster eines Slots: sichtbare
// Fenster vor versteckten, dann die groesste Flaeche. Hilfsfenster (DDE, IME,
// Tray-Popups) haben 0x0 oder winzige Rechtecke und fallen damit heraus.
func MainWindow(wins []window.Window) window.Window {
	if len(wins) == 0 {
		return nil
	}
	best := wins[0]
	bestVisible := best.IsVisible() && !best.IsMinimized()
	bestArea := areaOf(best)
	for _, w := range wins[1:] {
		vis := w.IsVisible() && !w.IsMinimized()
		area := areaOf(w)
		if vis && !bestVisible {
			best, bestVisible, bestArea = w, true, area
			continue
		}
		if vis == bestVisible && area > bestArea {
			best, bestArea = w, area
		}
	}
	return best
}

func areaOf(w window.Window) int {
	r, ok := w.Rect()
	if !ok {
		return 0
	}
	return r.Width() * r.Height()
}

// Find liefert die Fenster eines Slots aus einem Snapshot.
func (s Snapshot) Find(id string) []window.Window { return s.ByID[id] }
