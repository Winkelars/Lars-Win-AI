package window

// FakeWindow ist eine In-Memory-Implementierung von Window für Tests.
type FakeWindow struct {
	Hwnd        uintptr
	WindowTitle string
	Process     string
	Pid         uint32
	WindowClass string
	OwnerHwnd   uintptr
	Foreground  bool
	Minimized   bool
	Hidden      bool
	AlphaValue  int

	Monitor         int
	RestoreCalls    int
	MinimizeCalls   int
	HideCalls       int
	ShowNormalCalls int
	FocusCalls      int
	AlphaCalls      int
	MoveCalls       []int

	WindowRect Rect
	HasRect    bool

	Err error
}

// NewFakeWindow erstellt ein FakeWindow mit Default-Handle.
func NewFakeWindow(title, process string) *FakeWindow {
	return &FakeWindow{Hwnd: 1000, WindowTitle: title, Process: process, AlphaValue: 255}
}

// Handle liefert das Fake-Handle.
func (f *FakeWindow) Handle() uintptr { return f.Hwnd }

// Title liefert den Fenstertitel.
func (f *FakeWindow) Title() string { return f.WindowTitle }

// ProcessName liefert den Prozessnamen.
func (f *FakeWindow) ProcessName() string { return f.Process }

// PID liefert die Fake-Prozess-ID.
func (f *FakeWindow) PID() uint32 { return f.Pid }

// ClassName liefert die Fensterklasse.
func (f *FakeWindow) ClassName() string { return f.WindowClass }

// Owner liefert das Besitzer-Fenster (0 = keins).
func (f *FakeWindow) Owner() uintptr { return f.OwnerHwnd }

// IsForeground meldet, ob das Fenster im Vordergrund ist.
func (f *FakeWindow) IsForeground() bool { return f.Foreground }

// IsMinimized meldet, ob das Fenster minimiert ist.
func (f *FakeWindow) IsMinimized() bool { return f.Minimized }

// IsVisible meldet, ob das Fenster sichtbar ist.
func (f *FakeWindow) IsVisible() bool { return !f.Hidden }

// Restore macht das Fenster wieder sichtbar.
func (f *FakeWindow) Restore() error {
	f.RestoreCalls++
	f.Minimized = false
	return f.Err
}

// Minimize versteckt das Fenster (verhält sich wie Hide).
func (f *FakeWindow) Minimize() error {
	f.MinimizeCalls++
	f.Minimized = true
	f.Hidden = true
	f.Foreground = false
	return f.Err
}

// Hide versteckt das Fenster ohne Minimieren.
func (f *FakeWindow) Hide() error {
	f.HideCalls++
	f.Hidden = true
	f.Foreground = false
	return f.Err
}

// ShowNormal stellt das Fenster normal dar.
func (f *FakeWindow) ShowNormal() error {
	f.ShowNormalCalls++
	f.Minimized = false
	f.Hidden = false
	return f.Err
}

// Focus holt das Fenster in den Vordergrund.
func (f *FakeWindow) Focus() error {
	f.FocusCalls++
	f.Foreground = true
	f.Hidden = false
	return f.Err
}

// SetAlpha merkt sich den angeforderten Alphawert.
func (f *FakeWindow) SetAlpha(alpha uint8) error {
	f.AlphaCalls++
	f.AlphaValue = int(alpha)
	return f.Err
}

// Alpha liefert den aktuellen Alphawert (255 = deckend).
func (f *FakeWindow) Alpha() int {
	if f.AlphaValue == 0 {
		return 255
	}
	return f.AlphaValue
}

// Rect liefert das Fake-Rechteck, falls gesetzt.
func (f *FakeWindow) Rect() (Rect, bool) { return f.WindowRect, f.HasRect }

// MoveAndMaximize merkt sich den Zielmonitor und die Aufrufe.
func (f *FakeWindow) MoveAndMaximize(monitor int) error {
	f.MoveCalls = append(f.MoveCalls, monitor)
	f.Monitor = monitor
	return f.Err
}

// FakeManager ist eine In-Memory-Implementierung von Manager für Tests.
// FindTitleFn/FindProcessFn erlauben es, Aufrufe gezielt zu steuern.
type FakeManager struct {
	Windows       []*FakeWindow
	FindTitleFn   func(title string) (Window, bool)
	FindProcessFn func(process string) (Window, bool)
}

// NewFakeManager erstellt einen Manager mit den übergebenen Fenstern.
func NewFakeManager(windows ...*FakeWindow) *FakeManager {
	return &FakeManager{Windows: windows}
}

// FindByProcess sucht nach Prozessnamen.
func (m *FakeManager) FindByProcess(process string) (Window, bool) {
	if m.FindProcessFn != nil {
		return m.FindProcessFn(process)
	}
	for _, w := range m.Windows {
		if ProcessMatches(w.Process, process) {
			return w, true
		}
	}
	return nil, false
}

// FindByTitle sucht nach Fenstertitel.
func (m *FakeManager) FindByTitle(title string) (Window, bool) {
	if m.FindTitleFn != nil {
		return m.FindTitleFn(title)
	}
	for _, w := range m.Windows {
		if TitleMatches(w.WindowTitle, title) {
			return w, true
		}
	}
	return nil, false
}

// Enumerate liefert alle bekannten Fenster.
func (m *FakeManager) Enumerate() ([]Window, error) {
	out := make([]Window, 0, len(m.Windows))
	for _, w := range m.Windows {
		out = append(out, w)
	}
	return out, nil
}

// Add fügt ein Fenster hinzu.
func (m *FakeManager) Add(w *FakeWindow) { m.Windows = append(m.Windows, w) }
