package apps

import (
	"context"
	"fmt"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

// AgentIntegration kapselt den Herdr-Fokus (Pane/Tab/Workspace). Der Slot
// zeigt das Fenster zuerst; der Agent-Fokus laeuft danach im Hintergrund.
type AgentIntegration interface {
	Focus(ctx context.Context) error
}

// Deps buendelt die Abhaengigkeiten der Slot-Logik.
type Deps struct {
	Windows window.Manager
	Specs   []Spec
	Agent   AgentIntegration
	// Start startet einen Prozess (Seam fuer Tests).
	Start func(argv []string) error
	// HideOthers blendet beim Zeigen alle anderen Slots aus. Der Daemon will
	// das; der Bench laesst es standardmaessig weg, damit er nicht den
	// Bildschirm des Benutzers leerraeumt.
	HideOthers bool
	// AlphaEnabled meldet, ob kuenstliche Transparenz aktiv ist. nil = ja.
	AlphaEnabled func() bool
	// Sleep ist ein Seam fuer Tests; Default ist time.Sleep.
	Sleep func(time.Duration)
	// Timeout ist die Wartezeit auf ein frisch gestartetes Fenster.
	Timeout time.Duration
	Logf    func(format string, args ...any)
}

// Result beschreibt, was ein Toggle getan hat.
type Result int

const (
	// ResultNone: nichts zu tun.
	ResultNone Result = iota
	// ResultStart: Slot war nicht gestartet und wurde gestartet + gezeigt.
	ResultStart
	// ResultShow: Slot war inaktiv und wurde gezeigt.
	ResultShow
	// ResultHide: Slot war aktiv und wurde versteckt.
	ResultHide
)

// String liefert eine lesbare Bezeichnung.
func (r Result) String() string {
	switch r {
	case ResultStart:
		return "start"
	case ResultShow:
		return "show"
	case ResultHide:
		return "hide"
	default:
		return "none"
	}
}

func (d Deps) logf(format string, args ...any) {
	if d.Logf != nil {
		d.Logf(format, args...)
	}
}

// Toggle schaltet einen Slot um: nicht gestartet -> starten und zeigen,
// inaktiv -> zeigen (alle anderen Slots verstecken), aktiv -> verstecken.
func Toggle(ctx context.Context, d Deps, spec Spec) (Result, error) {
	if d.Windows == nil {
		return ResultNone, fmt.Errorf("apps: kein Fenster-Manager")
	}
	if !spec.IsValid() {
		return ResultNone, fmt.Errorf("apps[%s]: process, class oder title muss gesetzt sein", spec.ID)
	}

	snap, err := Take(d.Windows, d.Specs)
	if err != nil {
		return ResultNone, fmt.Errorf("apps[%s]: Fenster lesen: %w", spec.ID, err)
	}
	wins := snap.Find(spec.ID)

	if len(wins) == 0 {
		if err := launch(d, spec); err != nil {
			return ResultStart, err
		}
		snap, err = waitForWindow(ctx, d, spec)
		if err != nil {
			return ResultStart, err
		}
		return ResultStart, show(ctx, d, spec, snap.Find(spec.ID), snap)
	}

	if isActive(wins) {
		hideAll(wins)
		return ResultHide, nil
	}
	return ResultShow, show(ctx, d, spec, wins, snap)
}

// show macht den Slot sichtbar und fokussiert das Hauptfenster, danach werden
// alle anderen Slots versteckt. Die Reihenfolge verhindert ein kurzes
// Durchblitzen des Desktops.
func show(ctx context.Context, d Deps, spec Spec, wins []window.Window, snap Snapshot) error {
	if len(wins) == 0 {
		return fmt.Errorf("apps[%s]: kein Fenster zum Zeigen", spec.ID)
	}
	main := MainWindow(wins)
	if main == nil {
		return fmt.Errorf("apps[%s]: kein Fenster zum Zeigen", spec.ID)
	}
	for _, w := range wins {
		if w.IsMinimized() {
			_ = w.Restore()
		}
	}
	if err := main.MoveAndMaximize(spec.Monitor); err != nil {
		d.logf("apps[%s]: MoveAndMaximize: %v", spec.ID, err)
	}
	if d.AlphaEnabled == nil || d.AlphaEnabled() {
		applyAlpha(snap.Direct[spec.ID], spec.Alpha)
	}
	if err := main.Focus(); err != nil {
		return fmt.Errorf("apps[%s]: Fokus: %w", spec.ID, err)
	}
	// Nach SW_HIDE bleibt Windows manchmal auf einem versteckten Hilfsfenster
	// desselben Prozesses haengen oder die App meldet sich kurz als minimiert.
	// Deshalb kurz nachfassen, statt einmalig zu hoffen.
	for i := 0; i < 5; i++ {
		if main.IsForeground() && !main.IsMinimized() {
			break
		}
		time.Sleep(3 * time.Millisecond)
		if main.IsMinimized() {
			_ = main.Restore()
		}
		_ = main.Focus()
	}
	if d.HideOthers {
		for id, others := range snap.ByID {
			if id == spec.ID {
				continue
			}
			hideAll(others)
		}
	}
	if spec.IsHerdr() && d.Agent != nil {
		go focusAgent(ctx, d, spec.ID)
	}
	if spec.IsHerdr() {
		// WezTerm setzt seine Geometrie nach dem Erscheinen noch selbst und
		// ueberschreibt den ersten MoveAndMaximize. Deshalb wird die Position
		// im Hintergrund nachgezogen (idempotent, no-op wenn sie schon passt).
		go settle(ctx, d, spec)
	}
	return nil
}

// settle zieht die Fensterposition nach dem Zeigen mehrfach nach.
func settle(ctx context.Context, d Deps, spec Spec) {
	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(250 * time.Millisecond):
		}
		snap, err := Take(d.Windows, d.Specs)
		if err != nil {
			return
		}
		wins := snap.Find(spec.ID)
		main := MainWindow(wins)
		if main == nil || !main.IsVisible() {
			return
		}
		_ = main.MoveAndMaximize(spec.Monitor)
	}
}

// focusAgent laeuft nach dem Zeigen: der Herdr-Pane-Fokus kann mehrere
// Sekunden brauchen (IPC-Retries) und darf den Wechsel nicht blockieren.
func focusAgent(parent context.Context, d Deps, id string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 20*time.Second)
	defer cancel()
	if err := d.Agent.Focus(ctx); err != nil {
		d.logf("apps[%s]: Agent-Fokus: %v", id, err)
	}
}

// Hide versteckt einen Slot ohne Toggle-Logik (Bench, Aufraeumen).
func Hide(d Deps, spec Spec) error {
	snap, err := Take(d.Windows, d.Specs)
	if err != nil {
		return fmt.Errorf("apps[%s]: Fenster lesen: %w", spec.ID, err)
	}
	hideAll(snap.Find(spec.ID))
	return nil
}

// ApplyAlpha schaltet die kuenstliche Transparenz aller Slots um und liefert
// die Anzahl der angefassten Fenster. Slots ohne eigenes Alpha (Terminal,
// WezTerm, Pear Desktop) bleiben unberuehrt.
func ApplyAlpha(d Deps, enabled bool) int {
	snap, err := Take(d.Windows, d.Specs)
	if err != nil {
		return 0
	}
	count := 0
	for _, spec := range d.Specs {
		if spec.Alpha <= 0 || spec.Alpha >= AlphaOff {
			continue
		}
		value := uint8(spec.Alpha)
		if !enabled {
			value = AlphaOff
		}
		for _, w := range snap.Direct[spec.ID] {
			_ = w.SetAlpha(value)
			count++
		}
	}
	return count
}

// hideAll versteckt alle Fenster eines Slots (SW_HIDE, ohne Animation).
func hideAll(wins []window.Window) {
	for _, w := range wins {
		if w.IsVisible() {
			_ = w.Hide()
		}
	}
}

// isActive meldet, ob eines der Fenster sichtbar im Vordergrund ist.
func isActive(wins []window.Window) bool {
	for _, w := range wins {
		if w.IsForeground() && w.IsVisible() && !w.IsMinimized() {
			return true
		}
	}
	return false
}

// applyAlpha setzt die Deckkraft nur, wenn sie abweicht. 0 und 255 lassen
// native Transparenz unangetastet.
func applyAlpha(wins []window.Window, alpha int) {
	if alpha <= 0 || alpha >= AlphaOff {
		return
	}
	for _, w := range wins {
		if w.Alpha() == alpha {
			continue
		}
		_ = w.SetAlpha(uint8(alpha))
	}
}

func launch(d Deps, spec Spec) error {
	if len(spec.Launch) == 0 {
		return fmt.Errorf("apps[%s]: nicht gestartet und kein launch hinterlegt", spec.ID)
	}
	if d.Start == nil {
		return fmt.Errorf("apps[%s]: kein Start-Handler", spec.ID)
	}
	d.logf("apps[%s]: starte %v", spec.ID, spec.Launch)
	return d.Start(spec.Launch)
}

// waitForWindow pollt, bis das gestartete Fenster erscheint.
func waitForWindow(ctx context.Context, d Deps, spec Spec) (Snapshot, error) {
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	sleep := d.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	deadline := time.Now().Add(timeout)
	for {
		snap, err := Take(d.Windows, d.Specs)
		if err == nil && len(snap.Find(spec.ID)) > 0 {
			return snap, nil
		}
		if time.Now().After(deadline) {
			return Snapshot{}, fmt.Errorf("apps[%s]: Fenster erschien nicht innerhalb %s", spec.ID, timeout)
		}
		select {
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		default:
		}
		sleep(50 * time.Millisecond)
	}
}
