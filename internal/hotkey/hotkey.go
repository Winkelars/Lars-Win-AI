// Package hotkey beschreibt den WH_KEYBOARD_LL-Hook hinter einem testbaren
// Interface. Die Erkennung (Matches/Detector) ist rein und damit ohne echten
// Hook testbar. Die OS-Implementierung liegt in internal/platform.
package hotkey

import "context"

// Event ist ein rohes Tastatur-Ereignis aus dem Low-Level-Hook.
type Event struct {
	ScanCode uint32
	KeyDown  bool
	Alt      bool
	Ctrl     bool
	Shift    bool
	Win      bool
	Injected bool
}

// Config steuert, welche Taste als Toggle-Hotkey gilt.
type Config struct {
	ScanCode     uint32
	AltOnly      bool
	ExcludeAltGr bool
}

// Hook liefert Tastatur-Ereignisse über einen Channel, bis ctx beendet wird.
type Hook interface {
	Run(ctx context.Context, events chan<- Event) error
}

// Matches prüft, ob ein Event den konfigurierten Toggle-Hotkey auslöst.
// Es werden nur passende KeyDown-Ereignisse akzeptiert; AltGr (Ctrl+Alt) und
// zusätzliche Modifier (Strg/Shift/Win) werden ausgeschlossen.
func Matches(e Event, cfg Config) bool {
	if !e.KeyDown || e.Injected {
		return false
	}
	if e.ScanCode != cfg.ScanCode {
		return false
	}
	if !e.Alt {
		return false
	}
	if cfg.ExcludeAltGr && e.Ctrl {
		return false
	}
	if cfg.AltOnly && (e.Ctrl || e.Shift || e.Win) {
		return false
	}
	return true
}

// Detector erkennt die steigende Flanke eines Hotkey-Drucks. Er ist zustands-
// behaftet, damit ein gehaltener Hotkey nur einmal auslöst.
type Detector struct {
	cfg  Config
	down bool
}

// NewDetector erstellt einen Detector für die gegebene Konfiguration.
func NewDetector(cfg Config) *Detector { return &Detector{cfg: cfg} }

// Feed verarbeitet ein Event und meldet true beim ersten passenden KeyDown.
func (d *Detector) Feed(e Event) bool {
	if e.ScanCode == d.cfg.ScanCode && !e.KeyDown {
		d.down = false
		return false
	}
	if Matches(e, d.cfg) {
		if d.down {
			return false
		}
		d.down = true
		return true
	}
	return false
}

// FakeHook ist ein Hook für Tests und den --no-hook-Betrieb.
type FakeHook struct {
	Events []Event
	Err    error
	Feed   func(ctx context.Context, out chan<- Event) error
}

// Run speist zuerst die vordefinierten Events ein und blockiert danach (bzw.
// ruft Feed) bis ctx beendet wird.
func (h *FakeHook) Run(ctx context.Context, out chan<- Event) error {
	for _, e := range h.Events {
		select {
		case out <- e:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if h.Feed != nil {
		return h.Feed(ctx, out)
	}
	if h.Err != nil {
		return h.Err
	}
	<-ctx.Done()
	return ctx.Err()
}

var _ Hook = (*FakeHook)(nil)
