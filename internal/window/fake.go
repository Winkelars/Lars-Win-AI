package window

// FakeWindow ist eine In-Memory-Implementierung von Window für Tests.
type FakeWindow struct {
	Hwnd        uintptr
	WindowTitle string
	Process     string
	Foreground  bool
	Minimized   bool

	Monitor         int
	RestoreCalls    int
	MinimizeCalls   int
	ShowNormalCalls int
	FocusCalls      int
	MoveCalls       []int

	Err error
}

// NewFakeWindow erstellt ein FakeWindow mit Default-Handle.
func NewFakeWindow(title, process string) *FakeWindow {
	return &FakeWindow{Hwnd: 1000, WindowTitle: title, Process: process}
}

// Handle liefert das Fake-Handle.
func (f *FakeWindow) Handle() uintptr { return f.Hwnd }

// Title liefert den Fenstertitel.
func (f *FakeWindow) Title() string { return f.WindowTitle }

// ProcessName liefert den Prozessnamen.
func (f *FakeWindow) ProcessName() string { return f.Process }

// IsForeground meldet, ob das Fenster im Vordergrund ist.
func (f *FakeWindow) IsForeground() bool { return f.Foreground }

// IsMinimized meldet, ob das Fenster minimiert ist.
func (f *FakeWindow) IsMinimized() bool { return f.Minimized }

// Restore macht das Fenster wieder sichtbar.
func (f *FakeWindow) Restore() error {
	f.RestoreCalls++
	f.Minimized = false
	return f.Err
}

// Minimize minimiert das Fenster.
func (f *FakeWindow) Minimize() error {
	f.MinimizeCalls++
	f.Minimized = true
	f.Foreground = false
	return f.Err
}

// ShowNormal stellt das Fenster normal dar.
func (f *FakeWindow) ShowNormal() error {
	f.ShowNormalCalls++
	f.Minimized = false
	return f.Err
}

// Focus holt das Fenster in den Vordergrund.
func (f *FakeWindow) Focus() error {
	f.FocusCalls++
	f.Foreground = true
	return f.Err
}

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
