// Command aid ist der Lars-Win-AI-Daemon. CLI-Dispatch exakt nach
// docs/CONTRACTS.md §5. Als windowsgui-Binary gibt er im Hintergrund nichts
// auf der Konsole aus; die CLI-Unterbefehle schreiben bewusst nach stdout.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/apps"
	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/herdr"
	"github.com/Winkelars/Lars-Win-AI/internal/hotkey"
	"github.com/Winkelars/Lars-Win-AI/internal/logging"
	"github.com/Winkelars/Lars-Win-AI/internal/platform"
	"github.com/Winkelars/Lars-Win-AI/internal/runstate"
	"github.com/Winkelars/Lars-Win-AI/internal/task"
	"github.com/Winkelars/Lars-Win-AI/internal/toggle"
	"github.com/Winkelars/Lars-Win-AI/internal/window"
)

const version = "0.2.0"

// actionAlpha ist die reservierte Hotkey-Action fuer den Transparenz-Toggle.
const actionAlpha = "alpha"

// trigger ist ein Arbeitsauftrag fuer den Slot-Worker.
type trigger struct {
	alpha bool
	spec  apps.Spec
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "aid:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "run":
		return cmdRun(args[1:])
	case "toggle":
		return cmdToggle(args[1:])
	case "app":
		return cmdApp(args[1:])
	case "apps":
		return cmdApps(args[1:])
	case "windows":
		return cmdWindows(args[1:])
	case "alpha":
		return cmdAlpha(args[1:])
	case "register":
		return cmdRegister(args[1:])
	case "unregister":
		return cmdUnregister(args[1:])
	case "status":
		return cmdStatus(args[1:])
	case "version", "--version", "-v":
		fmt.Println("aid " + version)
		return nil
	case "help", "--help", "-h":
		usage(os.Stdout)
		return nil
	default:
		usage(os.Stderr)
		return fmt.Errorf("unbekannter Befehl %q", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `aid - Lars-Win-AI Daemon

Verwendung:
  aid run       [--config PATH] [--no-hook]   Hook + Slot-Loop
  aid toggle    [--config PATH]               Herdr-Slot umschalten
  aid app <id>  [--config PATH]               einen App-Slot umschalten
  aid apps      [--config PATH]               Slots und Bindings auflisten
  aid windows   [--match TEXT]                Top-Level-Fenster auflisten
  aid alpha <on|off>                          Transparenz einmalig anwenden
  aid register  [--config PATH]               Task "At log on" anlegen
  aid unregister                              Task entfernen
  aid status    [--config PATH] [--json]      Config + Task + Hook-Status
  aid version | --version
  aid help | --help
`)
}

func parseFlags(name string, args []string, register func(*flag.FlagSet)) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	register(fs)
	return fs.Parse(args)
}

func cmdRun(args []string) error {
	var configPath string
	var noHook bool
	if err := parseFlags("run", args, func(fs *flag.FlagSet) {
		fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
		fs.BoolVar(&noHook, "no-hook", false, "keinen Tastatur-Hook installieren")
	}); err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	log := openLogger()
	defer log.Close()
	log.Infof("aid %s startet (config=%s, hook=%v)", version, cfg.Path, !noHook)

	release, err := runstate.Acquire(runstate.DefaultPath())
	if err != nil {
		log.Errorf("Start abgelehnt: %v", err)
		return err
	}
	defer release()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Animations {
		if prev, err := platform.AnimationsEnabled(); err == nil && prev {
			if err := platform.SetAnimations(false); err == nil {
				defer func() { _ = platform.SetAnimations(true) }()
				log.Infof("System-Animationen während der Laufzeit deaktiviert")
			}
		}
	}

	deps := newAppDeps(cfg, log)
	var alphaOn atomic.Bool
	alphaOn.Store(true)
	deps.AlphaEnabled = alphaOn.Load

	if noHook {
		log.Infof("--no-hook aktiv: warte auf Beendigungssignal")
		<-ctx.Done()
		log.Infof("aid beendet")
		return nil
	}

	set := hotkey.NewSet(bindingsFor(cfg))
	router := hotkey.NewRouter(set)
	events := make(chan hotkey.Event, 64)
	hookErr := make(chan error, 1)
	go func() { hookErr <- platform.NewHook(set).Run(ctx, events) }()

	specs := apps.Specs(cfg)
	triggers := make(chan trigger, 8)
	go slotWorker(ctx, triggers, deps, &alphaOn, log)

	log.Infof("Hook installiert (ScanCodes=%v)", set.ScanCodes())
	for {
		select {
		case <-ctx.Done():
			log.Infof("aid beendet")
			return nil
		case err := <-hookErr:
			if err != nil {
				return fmt.Errorf("Hook: %w", err)
			}
			log.Infof("aid beendet (Hook-Ende)")
			return nil
		case e := <-events:
			if e.ScanCode == cfg.Hotkey.ScanCode || isBound(set, e.ScanCode) {
				log.Debugf("hotkey evt sc=%d down=%v alt=%v ctrl=%v shift=%v win=%v inj=%v",
					e.ScanCode, e.KeyDown, e.Alt, e.Ctrl, e.Shift, e.Win, e.Injected)
			}
			binding, ok := router.Feed(e)
			if !ok {
				continue
			}
			if binding.Action == actionAlpha {
				select {
				case triggers <- trigger{alpha: true}:
				default:
					log.Warnf("Transparenz-Trigger verworfen (Worker belegt)")
				}
				continue
			}
			spec, found := specByAction(specs, binding.Action)
			if !found {
				log.Errorf("Hotkey %q: kein Slot gefunden", binding.Action)
				continue
			}
			select {
			case triggers <- trigger{spec: spec}:
			default:
				log.Warnf("Slot %q: Trigger verworfen (Worker belegt)", spec.ID)
			}
		}
	}
}

func isBound(set hotkey.Set, sc uint32) bool {
	for _, code := range set.ScanCodes() {
		if code == sc {
			return true
		}
	}
	return false
}

func slotWorker(ctx context.Context, triggers <-chan trigger, deps apps.Deps, alphaOn *atomic.Bool, log *logging.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-triggers:
			if t.alpha {
				enabled := !alphaOn.Load()
				alphaOn.Store(enabled)
				count := apps.ApplyAlpha(deps, enabled)
				log.Infof("Transparenz: %v (%d Fenster)", enabled, count)
				continue
			}
			result, err := apps.Toggle(ctx, deps, t.spec)
			if err != nil {
				log.Errorf("Toggle fehlgeschlagen (%s, %s): %v", t.spec.ID, result, err)
				continue
			}
			log.Infof("Toggle ausgeführt: %s (%s)", t.spec.ID, result)
		}
	}
}

func cmdToggle(args []string) error {
	var configPath string
	if err := parseFlags("toggle", args, func(fs *flag.FlagSet) {
		fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
	}); err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	log := openLogger()
	defer log.Close()

	result, err := apps.Toggle(context.Background(), newAppDeps(cfg, log), apps.Herdr(cfg))
	if err != nil {
		log.Errorf("Toggle fehlgeschlagen (%s): %v", result, err)
		return err
	}
	log.Infof("Toggle ausgeführt: %s (%s)", apps.HerdrID, result)
	fmt.Println(result)
	return nil
}

func cmdApp(args []string) error {
	var configPath string
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		return fmt.Errorf("aid app: Slot-ID fehlt")
	}
	id := strings.TrimSpace(rest[0])
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	spec, ok := specByAction(apps.Specs(cfg), id)
	if !ok {
		return fmt.Errorf("aid app: unbekannter Slot %q", id)
	}
	log := openLogger()
	defer log.Close()

	result, err := apps.Toggle(context.Background(), newAppDeps(cfg, log), spec)
	if err != nil {
		log.Errorf("Toggle fehlgeschlagen (%s): %v", result, err)
		return err
	}
	log.Infof("Toggle ausgeführt: %s (%s)", id, result)
	fmt.Println(result)
	return nil
}

func cmdApps(args []string) error {
	var configPath string
	if err := parseFlags("apps", args, func(fs *flag.FlagSet) {
		fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
	}); err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	fmt.Printf("%-10s %-9s %-8s %s\n", "ID", "KEY", "MONITOR", "MATCH")
	fmt.Printf("%-10s sc=%-6d %-8s %s\n", "alpha", cfg.AlphaHotkey.ScanCode, "-", "Transparenz an/aus")
	for _, spec := range apps.Specs(cfg) {
		fmt.Printf("%-10s sc=%-6d %-8d %s\n", spec.ID, spec.ScanCode, spec.Monitor, describeSpec(spec))
	}
	return nil
}

func cmdAlpha(args []string) error {
	var configPath string
	fs := flag.NewFlagSet("alpha", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("aid alpha: on oder off erwartet")
	}
	mode := strings.ToLower(strings.TrimSpace(rest[0]))
	if mode != "on" && mode != "off" {
		return fmt.Errorf("aid alpha: %q ist weder on noch off", mode)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	log := openLogger()
	defer log.Close()
	count := apps.ApplyAlpha(newAppDeps(cfg, log), mode == "on")
	fmt.Printf("Transparenz: %s (%d Fenster)\n", mode, count)
	return nil
}

func cmdWindows(args []string) error {
	var match string
	if err := parseFlags("windows", args, func(fs *flag.FlagSet) {
		fs.StringVar(&match, "match", "", "Prozess-, Klassen- oder Titelfilter")
	}); err != nil {
		return err
	}
	all, err := platform.NewWindowManager().Enumerate()
	if err != nil {
		return err
	}
	fmt.Printf("%-8s %-26s %-24s %-16s %-4s %-4s %-4s %-4s %s\n",
		"HWND", "PROCESS", "CLASS", "TITLE", "VIS", "FG", "MIN", "A", "RECT")
	for _, w := range all {
		title := w.Title()
		if title == "" {
			continue
		}
		process := w.ProcessName()
		class := w.ClassName()
		if match != "" &&
			!strings.Contains(strings.ToLower(title), strings.ToLower(match)) &&
			!strings.Contains(strings.ToLower(class), strings.ToLower(match)) &&
			!window.ProcessMatches(process, match) {
			continue
		}
		rect, _ := w.Rect()
		alpha := w.Alpha()
		alphaText := "-"
		if alpha != 255 {
			alphaText = fmt.Sprint(alpha)
		}
		fmt.Printf("%-8d %-26s %-24s %-16.16s %-4v %-4v %-4v %-4s %dx%d@%d,%d\n",
			w.Handle(), process, class, title, w.IsVisible(), w.IsForeground(), w.IsMinimized(), alphaText,
			rect.Width(), rect.Height(), rect.Left, rect.Top)
	}
	seen := map[string]bool{}
	for i := 1; i <= 3; i++ {
		r, ok := platform.MonitorWorkArea(i)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d,%d,%d,%d", r.Left, r.Top, r.Width(), r.Height())
		if seen[key] {
			break
		}
		seen[key] = true
		fmt.Printf("Monitor %d (Work Area): %dx%d @%d,%d\n", i, r.Width(), r.Height(), r.Left, r.Top)
	}
	return nil
}

func describeSpec(spec apps.Spec) string {
	parts := make([]string, 0, 3)
	if spec.Process != "" {
		parts = append(parts, "process="+spec.Process)
	}
	if spec.Class != "" {
		parts = append(parts, "class="+spec.Class)
	}
	if spec.Title != "" {
		parts = append(parts, "title="+spec.Title)
	}
	return strings.Join(parts, " ")
}

func cmdRegister(args []string) error {
	var configPath string
	if err := parseFlags("register", args, func(fs *flag.FlagSet) {
		fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
	}); err != nil {
		return err
	}
	if _, err := config.Load(configPath); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("eigener Pfad nicht ermittelbar: %w", err)
	}
	runArgs := []string{"run"}
	if configPath != "" {
		runArgs = append(runArgs, "--config", configPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := task.Register(ctx, exe, runArgs); err != nil {
		return err
	}
	fmt.Printf("Task %s%s registriert: %s %s\n", task.Dir, task.Name, exe, strings.Join(runArgs, " "))
	return nil
}

func cmdUnregister(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := task.Unregister(ctx); err != nil {
		return err
	}
	fmt.Printf("Task %s%s entfernt\n", task.Dir, task.Name)
	return nil
}

type statusOutput struct {
	Version       string   `json:"version"`
	Config        string   `json:"config"`
	WindowTitle   string   `json:"window_title"`
	ProcessName   string   `json:"process_name"`
	Monitor       int      `json:"monitor"`
	AgentName     string   `json:"agent_name"`
	AgentKind     string   `json:"agent_kind"`
	Apps          []string `json:"apps"`
	Task          string   `json:"task"`
	TaskInstalled bool     `json:"task_installed"`
	HookRunning   bool     `json:"hook_running"`
}

func cmdStatus(args []string) error {
	var configPath string
	var asJSON bool
	if err := parseFlags("status", args, func(fs *flag.FlagSet) {
		fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
		fs.BoolVar(&asJSON, "json", false, "Ausgabe als JSON")
	}); err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	installed, info, qerr := task.Query(ctx)
	taskInfo := info
	if qerr != nil {
		taskInfo = "error: " + qerr.Error()
	} else if !installed {
		taskInfo = "not-found"
	}

	ids := make([]string, 0, len(cfg.Apps))
	for _, a := range cfg.Apps {
		ids = append(ids, a.ID)
	}
	out := statusOutput{
		Version:       version,
		Config:        cfg.Path,
		WindowTitle:   cfg.WindowTitle,
		ProcessName:   cfg.ProcessName,
		Monitor:       cfg.Monitor,
		AgentName:     cfg.AgentName,
		AgentKind:     cfg.AgentKind,
		Apps:          ids,
		Task:          taskInfo,
		TaskInstalled: installed,
		HookRunning:   runstate.IsRunning(runstate.DefaultPath()),
	}

	if asJSON {
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("aid %s\n", out.Version)
	fmt.Printf("Config:        %s\n", out.Config)
	fmt.Printf("Herdr:         %s (%s) auf Monitor %d\n", out.WindowTitle, out.ProcessName, out.Monitor)
	fmt.Printf("Agent:         %s (kind=%s)\n", out.AgentName, out.AgentKind)
	fmt.Printf("Slots:         %s\n", strings.Join(out.Apps, ", "))
	fmt.Printf("Task:          %s (registriert=%v)\n", out.Task, installed)
	fmt.Printf("Hook läuft:    %v\n", out.HookRunning)
	return nil
}

// agentAdapter verbindet den Herdr-Slot mit der bestehenden opencode-Logik.
type agentAdapter struct {
	deps toggle.Deps
	cfg  *config.Config
}

func (a agentAdapter) Focus(ctx context.Context) error {
	return toggle.FocusOpencode(ctx, a.deps, a.cfg)
}

func newAppDeps(cfg *config.Config, log *logging.Logger) apps.Deps {
	mgr := platform.NewWindowManager()
	td := toggle.Deps{
		Herdr: herdr.New(cfg.HerdrPath, cfg.HerdrRetryCount,
			time.Duration(cfg.HerdrRetryMS)*time.Millisecond),
		CFG: cfg,
	}
	deps := apps.Deps{
		Windows:    mgr,
		Specs:      apps.Specs(cfg),
		Start:      platform.StartProcess,
		Timeout:    time.Duration(cfg.StartupTimeoutMS) * time.Millisecond,
		HideOthers: true,
	}
	if log != nil {
		td.Logf = log.Debugf
		deps.Logf = log.Debugf
	}
	deps.Agent = agentAdapter{deps: td, cfg: cfg}
	return deps
}

func bindingsFor(cfg *config.Config) []hotkey.Binding {
	h := cfg.Hotkey
	out := make([]hotkey.Binding, 0, len(cfg.Apps)+2)
	out = append(out, hotkey.Binding{
		Action:       apps.HerdrID,
		ScanCode:     h.ScanCode,
		AltOnly:      h.AltOnly,
		ExcludeAltGr: h.ExcludeAltGr,
	})
	out = append(out, hotkey.Binding{
		Action:       actionAlpha,
		ScanCode:     cfg.AlphaHotkey.ScanCode,
		AltOnly:      cfg.AlphaHotkey.AltOnly,
		ExcludeAltGr: cfg.AlphaHotkey.ExcludeAltGr,
	})
	for _, a := range cfg.Apps {
		out = append(out, hotkey.Binding{
			Action:       a.ID,
			ScanCode:     a.ScanCode,
			AltOnly:      true,
			ExcludeAltGr: true,
		})
	}
	return out
}

func specByAction(specs []apps.Spec, id string) (apps.Spec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return apps.Spec{}, false
}

func openLogger() *logging.Logger {
	log, err := logging.New(logging.DefaultPath(), logging.ParseLevel(os.Getenv("AID_LOG_LEVEL")))
	if err != nil {
		return logging.Nop()
	}
	return log
}
