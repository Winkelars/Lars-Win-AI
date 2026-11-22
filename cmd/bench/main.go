// Command bench misst die Slot-Umschaltung von aid auf einem echten Desktop.
// Es arbeitet in-process gegen dieselbe Logik wie der Daemon (internal/apps)
// und braucht deshalb weder Tastatur-Injektion noch einen laufenden Server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/apps"
	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/platform"
	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

const version = "0.1.0"

type options struct {
	configPath string
	outDir     string
	baseline   string
	slotA      string
	slotB      string
	iterations int
	timeoutMS  int
	maxShow    float64
	maxHide    float64
	maxSwitch  float64
	asJSON     bool
	dryRun     bool
	force      bool
	exclusive  bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func run() error {
	opt := options{}
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opt.configPath, "config", "", "Pfad zur config.json")
	fs.StringVar(&opt.outDir, "out", defaultOutDir(), "Verzeichnis für Reports")
	fs.StringVar(&opt.baseline, "baseline", "", "Report-JSON als Vergleichsbasis")
	fs.StringVar(&opt.slotA, "a", "", "erster Slot (Default: erster App-Slot)")
	fs.StringVar(&opt.slotB, "b", "", "zweiter Slot (Default: zweiter App-Slot)")
	fs.IntVar(&opt.iterations, "iterations", 20, "Messwiederholungen")
	fs.IntVar(&opt.timeoutMS, "timeout", 2000, "Zeitlimit je Messung in ms")
	fs.Float64Var(&opt.maxShow, "max-show", 50, "Grenze p95 show in ms")
	fs.Float64Var(&opt.maxHide, "max-hide", 30, "Grenze p95 hide in ms")
	fs.Float64Var(&opt.maxSwitch, "max-switch", 80, "Grenze p95 switch in ms")
	fs.BoolVar(&opt.asJSON, "json", false, "Report als JSON nach stdout")
	fs.BoolVar(&opt.dryRun, "dry-run", false, "nur Konfiguration prüfen")
	fs.BoolVar(&opt.force, "force", false, "auch Terminal-Hosts messen (kann den eigenen Bildschirm wegziehen)")
	fs.BoolVar(&opt.exclusive, "exclusive", false, "beim Zeigen auch alle anderen Slots verstecken (wie der Daemon)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	cfg, err := config.Load(opt.configPath)
	if err != nil {
		return err
	}
	specs := apps.Specs(cfg)

	slotA, slotB := opt.slotA, opt.slotB
	if slotA == "" || slotB == "" {
		safe := safeSlots(cfg.Apps)
		if len(safe) < 2 {
			return fmt.Errorf("weniger als zwei ungefährliche App-Slots; wähle --a/--b bewusst")
		}
		if slotA == "" {
			slotA = safe[0].ID
		}
		if slotB == "" {
			slotB = safe[1].ID
		}
	}
	specA, ok := lookup(specs, slotA)
	if !ok {
		return fmt.Errorf("unbekannter Slot %q", slotA)
	}
	specB, ok := lookup(specs, slotB)
	if !ok {
		return fmt.Errorf("unbekannter Slot %q", slotB)
	}
	if specA.ID == specB.ID {
		return fmt.Errorf("--a und --b müssen verschiedene Slots sein")
	}

	// Schutz: niemals das Fenster umschalten, in dem der Bench selbst (bzw.
	// ein Agent) läuft. Sonst zieht sich der Lauf den eigenen Bildschirm weg.
	for _, spec := range []apps.Spec{specA, specB} {
		if isHostProcess(spec.Process) && !opt.force {
			return fmt.Errorf("Slot %q (%s) ist ein Terminal-Host; --force nötig, wenn das wirklich gewollt ist",
				spec.ID, spec.Process)
		}
	}

	deps := apps.Deps{
		Windows:    platform.NewWindowManager(),
		Specs:      specs,
		Start:      platform.StartProcess,
		Timeout:    time.Duration(cfg.StartupTimeoutMS) * time.Millisecond,
		HideOthers: opt.exclusive,
	}

	if opt.dryRun {
		fmt.Printf("bench %s (dry-run)\n", version)
		fmt.Printf("config: %s\n", cfg.Path)
		fmt.Printf("slots:  %s -> sc=%d, %s -> sc=%d\n", specA.ID, specA.ScanCode, specB.ID, specB.ScanCode)
		return nil
	}

	// Ein Bench, der im Vordergrund ein Terminal vorfindet, arbeitet gegen den
	// Benutzer: das Zeigen eines Slots versteckt alle anderen, also auch das
	// Terminal, in dem der Agent laeuft.
	if !opt.force {
		if fg := platform.ForegroundProcessName(); isHostProcess(fg) {
			return fmt.Errorf("Vordergrund ist %q (Terminal). Bench nur bei freiem Desktop starten oder --force", fg)
		}
	}

	fmt.Printf("bench %s – messe %s <-> %s, %d Iterationen\n", version, specA.ID, specB.ID, opt.iterations)
	fmt.Println("Der Bench blendet echte Fenster ein und aus. Abbruch mit Ctrl+C.")
	time.Sleep(2 * time.Second)

	ctx := context.Background()
	report := report{
		Version:    version,
		When:       time.Now().Format(time.RFC3339),
		Config:     cfg.Path,
		Iterations: opt.iterations,
	}

	// Ausgangszustand merken, damit am Ende nichts Verstecktes zurückbleibt —
	// auch wenn eine Messung mit Fehler abbricht.
	initial := activeSlot(specs, deps)
	defer func() {
		if initial != "" {
			spec, ok := lookup(specs, initial)
			if ok && len(windowsOf(deps, spec.ID)) > 0 && !visible(deps, spec.ID) {
				_, _ = apps.Toggle(context.Background(), deps, spec)
			}
			return
		}
		for _, spec := range []apps.Spec{specA, specB} {
			_ = apps.Hide(deps, spec)
		}
	}()

	// Cold-Start messen, falls ein Slot noch gar nicht läuft.
	for _, spec := range []apps.Spec{specA, specB} {
		if len(windowsOf(deps, spec.ID)) > 0 {
			continue
		}
		start := time.Now()
		res, err := apps.Toggle(ctx, deps, spec)
		if err != nil {
			report.addCheck(spec.ID+" cold", err.Error(), false)
			continue
		}
		if res != apps.ResultStart {
			report.addCheck(spec.ID+" cold", "unerwartet: "+res.String(), false)
			continue
		}
		report.addMetric(spec.ID+" cold", []float64{msSince(start)}, 0)
		_ = hide(ctx, deps, spec)
	}

	// Beide Slots in den Zustand "läuft, ist versteckt" bringen.
	for _, spec := range []apps.Spec{specA, specB} {
		if err := prepare(ctx, deps, spec); err != nil {
			return fmt.Errorf("Vorbereitung %s: %w", spec.ID, err)
		}
	}

	showValues := make([]float64, 0, opt.iterations)
	hideValues := make([]float64, 0, opt.iterations)
	switchValues := make([]float64, 0, opt.iterations)

	for i := 0; i < opt.iterations; i++ {
		v, err := measure(ctx, deps, specA, apps.ResultShow, func() bool { return foreground(deps, specA.ID) }, opt.timeoutMS)
		if err != nil {
			return fmt.Errorf("show %s: %w", specA.ID, err)
		}
		showValues = append(showValues, v)

		v, err = measure(ctx, deps, specA, apps.ResultHide, func() bool { return allHidden(deps, specA.ID) }, opt.timeoutMS)
		if err != nil {
			return fmt.Errorf("hide %s: %w", specA.ID, err)
		}
		hideValues = append(hideValues, v)
	}

	// Wechsel A <-> B inkl. Flacker-Sampling. A wird zuerst sichtbar
	// gemacht, damit ein echter Wechsel und kein "aus dem Nichts" gemessen wird.
	if !visible(deps, specA.ID) {
		if _, err := apps.Toggle(ctx, deps, specA); err != nil {
			return fmt.Errorf("Vorbereitung Wechsel: %w", err)
		}
	}
	target := specB
	for i := 0; i < opt.iterations; i++ {
		before := append(windowsOf(deps, specA.ID), windowsOf(deps, specB.ID)...)
		sampler := startSampler(before)
		v, err := measure(ctx, deps, target, apps.ResultShow, func() bool { return foreground(deps, target.ID) }, opt.timeoutMS)
		sampler.stopAndWait()
		if err != nil {
			return fmt.Errorf("switch %s: %w", target.ID, err)
		}
		report.Flashes += sampler.flashes
		switchValues = append(switchValues, v)
		if target.ID == specA.ID {
			target = specB
		} else {
			target = specA
		}
	}

	report.addMetric("show", showValues, opt.maxShow)
	report.addMetric("hide", hideValues, opt.maxHide)
	report.addMetric("switch", switchValues, opt.maxSwitch)
	report.addCheck("flash", fmt.Sprintf("%d Ereignisse ohne sichtbaren Slot", report.Flashes), report.Flashes == 0)

	// Geometrie und Alpha prüfen.
	for _, spec := range []apps.Spec{specA, specB} {
		if err := checkGeometry(deps, spec, &report); err != nil {
			report.addCheck(spec.ID+" geometry", err.Error(), false)
		}
		checkAlpha(deps, spec, &report)
	}

	report.Passed = report.passed()

	if opt.asJSON {
		return writeJSON(os.Stdout, report)
	}
	printReport(report, opt)
	if !report.Passed {
		return fmt.Errorf("Bench-Grenzen verletzt")
	}
	return nil
}

func defaultOutDir() string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return filepath.Join(v, "Lars-Win-AI", "bench")
	}
	return "bench-results"
}

func lookup(specs []apps.Spec, id string) (apps.Spec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return apps.Spec{}, false
}

// isHostProcess meldet Prozesse, in denen typischerweise der Agent selbst
// läuft. Ein Bench, der so ein Fenster umschaltet, zieht sich den eigenen
// Bildschirm weg.
func isHostProcess(process string) bool {
	p := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(process), ".exe"))
	switch p {
	case "windowsterminal", "wezterm-gui", "wezterm", "conhost", "openconsole",
		"pwsh", "powershell", "cmd", "alacritty", "kitty", "ghostty", "mintty":
		return true
	}
	return false
}

// safeSlots filtert Terminal-Hosts aus den Default-Kandidaten.
func safeSlots(defs []config.App) []config.App {
	out := make([]config.App, 0, len(defs))
	for _, a := range defs {
		if isHostProcess(a.Process) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func windowsOf(deps apps.Deps, id string) []window.Window {
	snap, err := apps.Take(deps.Windows, deps.Specs)
	if err != nil {
		return nil
	}
	return snap.Find(id)
}

func visible(deps apps.Deps, id string) bool {
	for _, w := range windowsOf(deps, id) {
		if w.IsVisible() {
			return true
		}
	}
	return false
}

func foreground(deps apps.Deps, id string) bool {
	for _, w := range windowsOf(deps, id) {
		if w.IsForeground() && w.IsVisible() && !w.IsMinimized() {
			return true
		}
	}
	return false
}

func allHidden(deps apps.Deps, id string) bool {
	wins := windowsOf(deps, id)
	if len(wins) == 0 {
		return false
	}
	for _, w := range wins {
		if w.IsVisible() {
			return false
		}
	}
	return true
}

func activeSlot(specs []apps.Spec, deps apps.Deps) string {
	for _, s := range specs {
		if foreground(deps, s.ID) {
			return s.ID
		}
	}
	return ""
}

// prepare stellt sicher, dass der Slot läuft und versteckt ist.
func prepare(ctx context.Context, deps apps.Deps, spec apps.Spec) error {
	if len(windowsOf(deps, spec.ID)) == 0 {
		if _, err := apps.Toggle(ctx, deps, spec); err != nil {
			return err
		}
	}
	return apps.Hide(deps, spec)
}

func hide(ctx context.Context, deps apps.Deps, spec apps.Spec) error {
	return apps.Hide(deps, spec)
}

func measure(ctx context.Context, deps apps.Deps, spec apps.Spec, want apps.Result, cond func() bool, timeoutMS int) (float64, error) {
	start := time.Now()
	result, err := apps.Toggle(ctx, deps, spec)
	if err != nil {
		return 0, err
	}
	if result != want {
		return 0, fmt.Errorf("erwartet %s, war %s; Fenster: %s", want, result, dumpSlot(deps, spec.ID))
	}
	deadline := start.Add(time.Duration(timeoutMS) * time.Millisecond)
	for !cond() {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("Zielzustand nach %d ms nicht erreicht; %s", timeoutMS, dumpAll(deps))
		}
		time.Sleep(200 * time.Microsecond)
	}
	return msSince(start), nil
}

func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}

// dumpSlot beschreibt den Slot-Zustand fuer Fehlermeldungen.
func dumpSlot(deps apps.Deps, id string) string {
	wins := windowsOf(deps, id)
	parts := make([]string, 0, len(wins))
	for _, w := range wins {
		parts = append(parts, fmt.Sprintf("%d %s vis=%v fg=%v min=%v",
			w.Handle(), w.ClassName(), w.IsVisible(), w.IsForeground(), w.IsMinimized()))
	}
	if len(parts) == 0 {
		return "keine"
	}
	return strings.Join(parts, " | ")
}

// dumpAll beschreibt alle Slot-Fenster (Diagnose bei Timeouts).
func dumpAll(deps apps.Deps) string {
	snap, err := apps.Take(deps.Windows, deps.Specs)
	if err != nil {
		return "Snapshot fehlgeschlagen: " + err.Error()
	}
	parts := make([]string, 0, len(deps.Specs))
	for _, spec := range deps.Specs {
		wins := snap.Find(spec.ID)
		if len(wins) == 0 {
			continue
		}
		parts = append(parts, spec.ID+"=>"+dumpSlot(deps, spec.ID))
	}
	return strings.Join(parts, " || ")
}

// flashSampler beobachtet während eines Wechsels, wie viele Slot-Fenster
// sichtbar sind. 0 bedeutet: der Desktop hat durchgeblitzt.
type flashSampler struct {
	stop    chan struct{}
	done    chan struct{}
	wins    []window.Window
	flashes int
}

func startSampler(wins []window.Window) *flashSampler {
	s := &flashSampler{
		stop: make(chan struct{}),
		done: make(chan struct{}),
		wins: wins,
	}
	go func() {
		defer close(s.done)
		for {
			select {
			case <-s.stop:
				return
			default:
			}
			n := 0
			for _, w := range s.wins {
				if w.IsVisible() {
					n++
				}
			}
			if n == 0 {
				s.flashes++
			}
			time.Sleep(time.Millisecond)
		}
	}()
	return s
}

func (s *flashSampler) stopAndWait() {
	close(s.stop)
	<-s.done
}

func checkGeometry(deps apps.Deps, spec apps.Spec, report *report) error {
	if spec.Monitor < 1 {
		return fmt.Errorf("kein Monitor konfiguriert")
	}
	area, ok := platform.MonitorWorkArea(spec.Monitor)
	if !ok {
		return fmt.Errorf("Monitor %d nicht gefunden", spec.Monitor)
	}
	if !foreground(deps, spec.ID) {
		if _, err := apps.Toggle(context.Background(), deps, spec); err != nil {
			return err
		}
	}
	wins := windowsOf(deps, spec.ID)
	if len(wins) == 0 {
		return fmt.Errorf("kein Fenster")
	}
	main := apps.MainWindow(wins)
	rect, ok := main.Rect()
	if !ok {
		return fmt.Errorf("Fensterrechteck nicht lesbar")
	}
	detail := fmt.Sprintf("%dx%d @(%d,%d), erwartet %dx%d @(%d,%d)",
		rect.Width(), rect.Height(), rect.Left, rect.Top,
		area.Width(), area.Height(), area.Left, area.Top)
	pass := abs(rect.Left-area.Left) <= 2 && abs(rect.Top-area.Top) <= 2 &&
		abs(rect.Width()-area.Width()) <= 2 && abs(rect.Height()-area.Height()) <= 2
	report.addCheck(spec.ID+" geometry", detail, pass)
	return nil
}

func checkAlpha(deps apps.Deps, spec apps.Spec, report *report) {
	if spec.Alpha <= 0 || spec.Alpha >= apps.AlphaOff {
		report.addCheck(spec.ID+" alpha", "native Transparenz (nicht gesetzt)", true)
		return
	}
	wins := windowsOf(deps, spec.ID)
	if len(wins) == 0 {
		report.addCheck(spec.ID+" alpha", "kein Fenster", false)
		return
	}
	got := apps.MainWindow(wins).Alpha()
	report.addCheck(spec.ID+" alpha", fmt.Sprintf("%d (erwartet %d)", got, spec.Alpha), got == spec.Alpha)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type metricResult struct {
	Name    string  `json:"name"`
	N       int     `json:"n"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	MaxMS   float64 `json:"max_ms"`
	LimitMS float64 `json:"limit_ms"`
	Pass    bool    `json:"pass"`
}

type checkResult struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Pass   bool   `json:"pass"`
}

type report struct {
	Version    string         `json:"version"`
	When       string         `json:"when"`
	Config     string         `json:"config"`
	Iterations int            `json:"iterations"`
	Metrics    []metricResult `json:"metrics"`
	Checks     []checkResult  `json:"checks"`
	Flashes    int            `json:"flashes"`
	Passed     bool           `json:"passed"`
}

func (r *report) addMetric(name string, values []float64, limit float64) {
	if len(values) == 0 {
		return
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	m := metricResult{
		Name:    name,
		N:       len(sorted),
		P50MS:   round(percentile(sorted, 0.50)),
		P95MS:   round(percentile(sorted, 0.95)),
		MaxMS:   round(sorted[len(sorted)-1]),
		LimitMS: limit,
	}
	m.Pass = limit <= 0 || m.P95MS <= limit
	r.Metrics = append(r.Metrics, m)
}

func (r *report) addCheck(name, detail string, pass bool) {
	r.Checks = append(r.Checks, checkResult{Name: name, Detail: detail, Pass: pass})
}

func (r report) passed() bool {
	for _, m := range r.Metrics {
		if !m.Pass {
			return false
		}
	}
	for _, c := range r.Checks {
		if !c.Pass {
			return false
		}
	}
	return true
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p*float64(len(sorted))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func round(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func printReport(r report, opt options) {
	fmt.Println()
	fmt.Printf("%-14s %5s %9s %9s %9s %9s  %s\n", "METRIK", "N", "P50", "P95", "MAX", "LIMIT", "")
	for _, m := range r.Metrics {
		limit := "-"
		if m.LimitMS > 0 {
			limit = fmt.Sprintf("%.0f", m.LimitMS)
		}
		status := "PASS"
		if !m.Pass {
			status = "FAIL"
		}
		fmt.Printf("%-14s %5d %8.1f %9.1f %9.1f %9s  %s\n", m.Name, m.N, m.P50MS, m.P95MS, m.MaxMS, limit, status)
	}
	fmt.Println()
	for _, c := range r.Checks {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
		}
		fmt.Printf("%-14s %-4s %s\n", c.Name, status, c.Detail)
	}
	fmt.Println()
	if r.Passed {
		fmt.Println("Ergebnis: PASS")
	} else {
		fmt.Println("Ergebnis: FAIL")
	}
	if opt.outDir != "" {
		path, err := saveReport(r, opt.outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Report nicht gespeichert: %v\n", err)
		} else {
			fmt.Printf("Report: %s\n", path)
		}
	}
	if opt.baseline != "" {
		printBaseline(r, opt.baseline)
	}
}

func saveReport(r report, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := "bench-" + time.Now().Format("20060102-150405") + ".json"
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := writeJSON(f, r); err != nil {
		return "", err
	}
	return path, nil
}

func writeJSON(w io.Writer, r report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func printBaseline(current report, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Baseline nicht lesbar: %v\n", err)
		return
	}
	var base report
	if err := json.Unmarshal(data, &base); err != nil {
		fmt.Fprintf(os.Stderr, "Baseline ungültig: %v\n", err)
		return
	}
	byName := make(map[string]float64, len(base.Metrics))
	for _, m := range base.Metrics {
		byName[m.Name] = m.P95MS
	}
	fmt.Println("Vergleich zur Baseline (p95):")
	for _, m := range current.Metrics {
		old, ok := byName[m.Name]
		if !ok {
			continue
		}
		fmt.Printf("  %-12s %s%+.1f ms\n", m.Name, sign(m.P95MS-old), m.P95MS-old)
	}
}

func sign(v float64) string {
	if v > 0 {
		return "+"
	}
	return ""
}
