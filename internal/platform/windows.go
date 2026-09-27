//go:build windows

// Package platform kapselt alle Windows-Syscalls (user32/kernel32) hinter
// build-tags. Nur Go-Standardbibliothek, kein golang.org/x/sys.
package platform

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Winkelars/Lars-Win-AI/internal/hotkey"
	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procIsZoomed                 = user32.NewProc("IsZoomed")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procSwitchToThisWindow       = user32.NewProc("SwitchToThisWindow")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procPeekMessageW             = user32.NewProc("PeekMessageW")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procMonitorFromPoint         = user32.NewProc("MonitorFromPoint")
	procEnumDisplayMonitors      = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
	procSystemParametersInfoW    = user32.NewProc("SystemParametersInfoW")
	procSetWindowsHookExW        = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx           = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procPostThreadMessageW       = user32.NewProc("PostThreadMessageW")

	procGetCurrentThreadID         = kernel32.NewProc("GetCurrentThreadId")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procGetExitCodeProcess         = kernel32.NewProc("GetExitCodeProcess")
)

const (
	swHide       = 0
	swShowNormal = 1
	swMaximize   = 3
	swMinimize   = 6
	swRestore    = 9

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	monitorInfofPrimary = 0x0001

	monitorDefaultToPrimary = 0x00000001

	processQueryLimitedInformation = 0x1000

	stillActive = 259

	spiGetAnimation = 0x1042
	spiSetAnimation = 0x1043

	whKeyboardLL = 13

	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012

	llkhfInjected = 0x10

	keyeventfKeyUp = 0x0002

	vkShift    = 0x10
	vkControl  = 0x11
	vkMenu     = 0x12
	vkLShift   = 0xA0
	vkRShift   = 0xA1
	vkLControl = 0xA2
	vkRControl = 0xA3
	vkLMenu    = 0xA4
	vkRMenu    = 0xA5
	vkLWin     = 0x5B
	vkRWin     = 0x5C
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

type animationInfo struct {
	Size       uint32
	MinAnimate int32
}

type kbdLLHookStruct struct {
	VkCode    uint32
	ScanCode  uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// NewWindowManager liefert die echte Windows-Fensterverwaltung.
func NewWindowManager() window.Manager { return &windowsManager{} }

type windowsManager struct{}

func (m *windowsManager) Enumerate() ([]window.Window, error) {
	var out []window.Window
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if hwnd == 0 {
			return 1
		}
		out = append(out, newWinWindow(syscall.Handle(hwnd)))
		return 1
	})
	procEnumWindows.Call(cb, 0)
	runtime.KeepAlive(cb)
	return out, nil
}

func (m *windowsManager) FindByTitle(title string) (window.Window, bool) {
	if strings.TrimSpace(title) == "" {
		return nil, false
	}
	for _, w := range m.visibleTitled() {
		if window.TitleMatches(w.Title(), title) {
			return w, true
		}
	}
	return nil, false
}

func (m *windowsManager) FindByProcess(processName string) (window.Window, bool) {
	if strings.TrimSpace(processName) == "" {
		return nil, false
	}
	for _, w := range m.visibleTitled() {
		if window.ProcessMatches(w.ProcessName(), processName) {
			return w, true
		}
	}
	return nil, false
}

func (m *windowsManager) visibleTitled() []*winWindow {
	all, _ := m.Enumerate()
	out := make([]*winWindow, 0, len(all))
	for _, w := range all {
		ww, ok := w.(*winWindow)
		if !ok {
			continue
		}
		if !ww.isVisible() || ww.Title() == "" {
			continue
		}
		out = append(out, ww)
	}
	return out
}

type winWindow struct{ hwnd syscall.Handle }

func newWinWindow(h syscall.Handle) *winWindow { return &winWindow{hwnd: h} }

func (w *winWindow) Handle() uintptr { return uintptr(w.hwnd) }

func (w *winWindow) Title() string {
	n, _, _ := procGetWindowTextLengthW.Call(uintptr(w.hwnd))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func (w *winWindow) ProcessName() string {
	var pid uint32
	procGetWindowThreadProcessID.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	return processNameForPID(pid)
}

func (w *winWindow) isVisible() bool {
	ret, _, _ := procIsWindowVisible.Call(uintptr(w.hwnd))
	return ret != 0
}

func (w *winWindow) isIconic() bool {
	ret, _, _ := procIsIconic.Call(uintptr(w.hwnd))
	return ret != 0
}

func (w *winWindow) isZoomed() bool {
	ret, _, _ := procIsZoomed.Call(uintptr(w.hwnd))
	return ret != 0
}

func (w *winWindow) IsForeground() bool {
	fg, _, _ := procGetForegroundWindow.Call()
	return fg == uintptr(w.hwnd)
}

func (w *winWindow) IsMinimized() bool { return w.isIconic() }

func (w *winWindow) Restore() error {
	if w.isIconic() {
		procShowWindow.Call(uintptr(w.hwnd), swRestore)
	}
	return nil
}

func (w *winWindow) Minimize() error {
	procShowWindow.Call(uintptr(w.hwnd), swMinimize)
	return nil
}

func (w *winWindow) ShowNormal() error {
	procShowWindow.Call(uintptr(w.hwnd), swShowNormal)
	return nil
}

func (w *winWindow) Focus() error {
	hwnd := uintptr(w.hwnd)
	if w.isIconic() {
		procShowWindow.Call(hwnd, swRestore)
		// Warten, bis die Wiederherstellung abgeschlossen ist; sonst schlaegt
		// SetForegroundWindow gegen ein noch ikonisches Fenster fehl.
		for i := 0; i < 25 && w.isIconic(); i++ {
			time.Sleep(8 * time.Millisecond)
		}
	}
	if foregroundIs(hwnd) {
		return nil
	}

	// An den aktuellen Foreground-Thread andocken: das hebt die Windows-
	// Foreground-Sperre auf. Der aufrufende Thread braucht eine Message-Queue.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var m msg
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 0)

	curThread, _, _ := procGetCurrentThreadID.Call()
	for attempt := 0; attempt < 3 && !foregroundIs(hwnd); attempt++ {
		fg, _, _ := procGetForegroundWindow.Call()
		fgThread, _, _ := procGetWindowThreadProcessID.Call(fg, 0)
		attached := false
		if fgThread != 0 && fgThread != curThread {
			if ret, _, _ := procAttachThreadInput.Call(fgThread, curThread, 1); ret != 0 {
				attached = true
			}
		}
		procBringWindowToTop.Call(hwnd)
		procSetForegroundWindow.Call(hwnd)
		if attached {
			procAttachThreadInput.Call(fgThread, curThread, 0)
		}
		if foregroundIs(hwnd) {
			return nil
		}
		// Fallback: simulierter Alt-Druck hebt die Foreground-Sperre auf.
		procKeybdEvent.Call(vkMenu, 0, 0, 0)
		procKeybdEvent.Call(vkMenu, 0, keyeventfKeyUp, 0)
		procSetForegroundWindow.Call(hwnd)
		time.Sleep(15 * time.Millisecond)
	}
	if !foregroundIs(hwnd) {
		procSwitchToThisWindow.Call(hwnd, 1)
	}
	return nil
}

func foregroundIs(hwnd uintptr) bool {
	fg, _, _ := procGetForegroundWindow.Call()
	return fg == hwnd
}

// MoveAndMaximize legt das Fenster randlos ueber den ARBEITSBEREICH des
// Zielmonitors (ohne die Taskleiste zu verdecken) und laesst es im
// Normal-Zustand ("windowed, ausgebreitet"). Bewusst KEIN SW_MAXIMIZE, weil
// ein Fenster ohne Dekoration sonst den kompletten Monitor (Vollbild) belegt.
func (w *winWindow) MoveAndMaximize(monitor int) error {
	r, ok := monitorRect(monitor)
	if !ok {
		if r, ok = primaryMonitorRect(); !ok {
			return fmt.Errorf("kein Monitor %d gefunden", monitor)
		}
	}
	if w.isIconic() || w.isZoomed() {
		procShowWindow.Call(uintptr(w.hwnd), swRestore)
	}
	width := r.Right - r.Left
	height := r.Bottom - r.Top
	procSetWindowPos.Call(
		uintptr(w.hwnd), 0,
		uintptr(int(r.Left)), uintptr(int(r.Top)),
		uintptr(int(width)), uintptr(int(height)),
		swpNoZOrder|swpNoActivate,
	)
	procShowWindow.Call(uintptr(w.hwnd), swShowNormal)
	return nil
}

func processNameForPID(pid uint32) string {
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		h, 0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 || size == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buf[:size])
	if idx := strings.LastIndexAny(full, `\/`); idx >= 0 {
		return full[idx+1:]
	}
	return full
}

func monitorRect(index int) (rect, bool) {
	type entry struct {
		r       rect
		primary bool
	}
	var mons []entry

	cb := syscall.NewCallback(func(hmon uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
		var mi monitorInfo
		mi.Size = uint32(unsafe.Sizeof(mi))
		if ret, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); ret != 0 {
			mons = append(mons, entry{r: mi.Work, primary: mi.Flags&monitorInfofPrimary != 0})
		}
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	runtime.KeepAlive(cb)

	if len(mons) == 0 {
		return rect{}, false
	}
	sort.SliceStable(mons, func(i, j int) bool {
		if mons[i].primary != mons[j].primary {
			return mons[i].primary
		}
		if mons[i].r.Left != mons[j].r.Left {
			return mons[i].r.Left < mons[j].r.Left
		}
		return mons[i].r.Top < mons[j].r.Top
	})
	if index < 1 {
		index = 1
	}
	if index > len(mons) {
		return mons[len(mons)-1].r, true
	}
	return mons[index-1].r, true
}

// primaryMonitorRect liefert das Rechteck des Primärmonitors über
// MonitorFromPoint(POINT{0,0}, MONITOR_DEFAULTTOPRIMARY).
func primaryMonitorRect() (rect, bool) {
	p := point{X: 0, Y: 0}
	hmon, _, _ := procMonitorFromPoint.Call(
		uintptr(unsafe.Pointer(&p)), monitorDefaultToPrimary,
	)
	if hmon == 0 {
		return rect{}, false
	}
	var mi monitorInfo
	mi.Size = uint32(unsafe.Sizeof(mi))
	if ret, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); ret == 0 {
		return rect{}, false
	}
	return mi.Work, true
}

// AnimationsEnabled liest die aktuelle System-Animationseinstellung.
func AnimationsEnabled() (bool, error) {
	var ai animationInfo
	ai.Size = uint32(unsafe.Sizeof(ai))
	ret, _, err := procSystemParametersInfoW.Call(
		spiGetAnimation, uintptr(ai.Size), uintptr(unsafe.Pointer(&ai)), 0,
	)
	if ret == 0 {
		return false, fmt.Errorf("SystemParametersInfo(SPI_GETANIMATION): %w", err)
	}
	return ai.MinAnimate != 0, nil
}

// SetAnimations setzt die System-Animationseinstellung (sessionweit).
func SetAnimations(enabled bool) error {
	var ai animationInfo
	ai.Size = uint32(unsafe.Sizeof(ai))
	ret, _, err := procSystemParametersInfoW.Call(
		spiGetAnimation, uintptr(ai.Size), uintptr(unsafe.Pointer(&ai)), 0,
	)
	if ret == 0 {
		return fmt.Errorf("SystemParametersInfo(SPI_GETANIMATION): %w", err)
	}
	want := int32(0)
	if enabled {
		want = 1
	}
	if ai.MinAnimate == want {
		return nil
	}
	ai.MinAnimate = want
	ret, _, err = procSystemParametersInfoW.Call(
		spiSetAnimation, uintptr(ai.Size), uintptr(unsafe.Pointer(&ai)), 0,
	)
	if ret == 0 {
		return fmt.Errorf("SystemParametersInfo(SPI_SETANIMATION): %w", err)
	}
	return nil
}

// ProcessAlive prüft, ob ein Prozess mit der PID noch läuft.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(uint32(pid)))
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)
	var code uint32
	ret, _, _ := procGetExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&code)))
	if ret == 0 {
		return false
	}
	return code == stillActive
}

// NewHook liefert den echten WH_KEYBOARD_LL-Hook.
func NewHook(cfg hotkey.Config) hotkey.Hook { return &windowsHook{cfg: cfg} }

// HideConsole verhindert, dass beim Starten von Konsolenprogrammen (z. B.
// herdr) aus dem GUI-Daemon heraus ein Konsolenfenster aufblitzt.
func HideConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

type windowsHook struct{ cfg hotkey.Config }

var (
	activeHookCallback uintptr
	activeHookProc     uintptr
	activeHookEvents   chan<- hotkey.Event
	activeHookCfg      hotkey.Config
)

func (h *windowsHook) Run(ctx context.Context, events chan<- hotkey.Event) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	activeHookEvents = events
	activeHookCfg = h.cfg
	activeHookCallback = syscall.NewCallback(lowLevelKeyboardProc)

	hHook, _, err := procSetWindowsHookExW.Call(whKeyboardLL, activeHookCallback, 0, 0)
	if hHook == 0 {
		return fmt.Errorf("SetWindowsHookExW(WH_KEYBOARD_LL): %w", err)
	}
	activeHookProc = hHook
	defer func() {
		procUnhookWindowsHookEx.Call(activeHookProc)
		activeHookProc = 0
		activeHookEvents = nil
	}()

	tid, _, _ := procGetCurrentThreadID.Call()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			procPostThreadMessageW.Call(tid, wmQuit, 0, 0)
		case <-done:
		}
	}()

	var m msg
	for {
		ret, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) == -1 {
			return fmt.Errorf("GetMessageW: %w", err)
		}
		if ret == 0 {
			return nil
		}
	}
}

func lowLevelKeyboardProc(nCode int, wParam uintptr, lParam unsafe.Pointer) uintptr {
	if nCode == 0 {
		k := (*kbdLLHookStruct)(lParam)
		down := wParam == wmKeyDown || wParam == wmSysKeyDown
		updateModifiers(k.VkCode, down)
		e := hotkey.Event{
			ScanCode: k.ScanCode,
			KeyDown:  down,
			// Modifier werden aus dem Hook-Stream selbst getrackt, weil
			// GetAsyncKeyState innerhalb eines Low-Level-Hooks unzuverlaessig
			// ist. Rechts-Alt (AltGr) erzeugt zusaetzlich ein synthetisches
			// Strg, daher wird es ueber modAltR mit ausgeschlossen.
			Alt:      modAltL || modAltR,
			Ctrl:     modCtrlL || modCtrlR || modAltR,
			Shift:    modShiftL || modShiftR,
			Win:      modWinL || modWinR,
			Injected: k.Flags&llkhfInjected != 0,
		}
		if activeHookEvents != nil {
			select {
			case activeHookEvents <- e:
			default:
			}
		}
		if hotkey.Matches(e, activeHookCfg) {
			return 1
		}
	}
	ret, _, _ := procCallNextHookEx.Call(activeHookProc, uintptr(nCode), wParam, uintptr(lParam))
	return ret
}

var (
	modAltL, modAltR     bool
	modCtrlL, modCtrlR   bool
	modShiftL, modShiftR bool
	modWinL, modWinR     bool
)

func updateModifiers(vk uint32, down bool) {
	switch vk {
	case vkMenu, vkLMenu:
		modAltL = down
	case vkRMenu:
		modAltR = down
	case vkControl, vkLControl:
		modCtrlL = down
	case vkRControl:
		modCtrlR = down
	case vkShift, vkLShift:
		modShiftL = down
	case vkRShift:
		modShiftR = down
	case vkLWin:
		modWinL = down
	case vkRWin:
		modWinR = down
	}
}

var (
	_ window.Manager = (*windowsManager)(nil)
	_ window.Window  = (*winWindow)(nil)
	_ hotkey.Hook    = (*windowsHook)(nil)
)
