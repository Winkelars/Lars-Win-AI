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
	"syscall"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/config"
	"github.com/Winkelars/Lars-Win-AI/internal/herdr"
	"github.com/Winkelars/Lars-Win-AI/internal/hotkey"
	"github.com/Winkelars/Lars-Win-AI/internal/logging"
	"github.com/Winkelars/Lars-Win-AI/internal/platform"
	"github.com/Winkelars/Lars-Win-AI/internal/runstate"
	"github.com/Winkelars/Lars-Win-AI/internal/task"
	"github.com/Winkelars/Lars-Win-AI/internal/toggle"
)

const version = "0.1.0"

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
  aid run       [--config PATH] [--no-hook]   Hook + Toggle-Loop
  aid register  [--config PATH]               Task "At log on" anlegen
  aid unregister                              Task entfernen
  aid status    [--config PATH] [--json]      Config + Task + Hook-Status
  aid toggle    [--config PATH] [--no-hook]   ein einzelner Toggle
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

	deps := newDeps(cfg)

	if noHook {
		log.Infof("--no-hook aktiv: warte auf Beendigungssignal")
		<-ctx.Done()
		log.Infof("aid beendet")
		return nil
	}

	hk := hotkey.Config{
		ScanCode:     cfg.Hotkey.ScanCode,
		AltOnly:      cfg.Hotkey.AltOnly,
		ExcludeAltGr: cfg.Hotkey.ExcludeAltGr,
	}
	events := make(chan hotkey.Event, 64)
	hookErr := make(chan error, 1)
	go func() { hookErr <- platform.NewHook(hk).Run(ctx, events) }()

	detector := hotkey.NewDetector(hk)
	triggers := make(chan struct{}, 1)
	go toggleWorker(ctx, triggers, deps, cfg, log)

	log.Infof("Hook installiert (ScanCode=%d)", hk.ScanCode)
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
			if detector.Feed(e) {
				select {
				case triggers <- struct{}{}:
				default:
				}
			}
		}
	}
}

func toggleWorker(ctx context.Context, triggers <-chan struct{}, deps toggle.Deps, cfg *config.Config, log *logging.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-triggers:
			action, err := toggle.Run(ctx, deps, cfg)
			if err != nil {
				log.Errorf("Toggle fehlgeschlagen (%s): %v", action, err)
				continue
			}
			log.Infof("Toggle ausgeführt: %s", action)
		}
	}
}

func cmdToggle(args []string) error {
	var configPath string
	var noHook bool
	if err := parseFlags("toggle", args, func(fs *flag.FlagSet) {
		fs.StringVar(&configPath, "config", "", "Pfad zur config.json")
		fs.BoolVar(&noHook, "no-hook", false, "Hook nicht verwenden (nur Test)")
	}); err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	log := openLogger()
	defer log.Close()

	action, err := toggle.Run(context.Background(), newDeps(cfg), cfg)
	if err != nil {
		log.Errorf("Toggle fehlgeschlagen (%s): %v", action, err)
		return err
	}
	log.Infof("Toggle ausgeführt: %s", action)
	fmt.Println(action)
	return nil
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
	Version       string `json:"version"`
	Config        string `json:"config"`
	WindowTitle   string `json:"window_title"`
	ProcessName   string `json:"process_name"`
	Monitor       int    `json:"monitor"`
	AgentName     string `json:"agent_name"`
	AgentKind     string `json:"agent_kind"`
	Task          string `json:"task"`
	TaskInstalled bool   `json:"task_installed"`
	HookRunning   bool   `json:"hook_running"`
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

	out := statusOutput{
		Version:       version,
		Config:        cfg.Path,
		WindowTitle:   cfg.WindowTitle,
		ProcessName:   cfg.ProcessName,
		Monitor:       cfg.Monitor,
		AgentName:     cfg.AgentName,
		AgentKind:     cfg.AgentKind,
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
	fmt.Printf("Fenster:       %s (%s) auf Monitor %d\n", out.WindowTitle, out.ProcessName, out.Monitor)
	fmt.Printf("Agent:         %s (kind=%s)\n", out.AgentName, out.AgentKind)
	fmt.Printf("Task:          %s (registriert=%v)\n", out.Task, installed)
	fmt.Printf("Hook läuft:    %v\n", out.HookRunning)
	return nil
}

func newDeps(cfg *config.Config) toggle.Deps {
	return toggle.Deps{
		Windows: platform.NewWindowManager(),
		Herdr: herdr.New(cfg.HerdrPath, cfg.HerdrRetryCount,
			time.Duration(cfg.HerdrRetryMS)*time.Millisecond),
		CFG: cfg,
	}
}

func openLogger() *logging.Logger {
	log, err := logging.New(logging.DefaultPath(), logging.ParseLevel(os.Getenv("AID_LOG_LEVEL")))
	if err != nil {
		return logging.Nop()
	}
	return log
}
