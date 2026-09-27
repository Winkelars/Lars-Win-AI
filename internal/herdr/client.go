// Package herdr kapselt die herdr-CLI als testbare Bridge
// (docs/CONTRACTS.md §6). Die JSON-Antworten liegen unter .result.*.
package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Winkelars/Lars-Win-AI/internal/platform"
)

// Agent spiegelt einen Eintrag aus `herdr agent list` (.result.agents[]).
type Agent struct {
	Label       string `json:"agent"` // Agenten-Label/Kind, z. B. "opencode"
	Name        string `json:"name"`  // eindeutiger Live-Name (kann leer sein)
	Kind        string `json:"kind"`  // optionales explizites Kind
	Status      string `json:"agent_status"`
	PaneID      string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Focused     bool   `json:"focused"`
}

// Target liefert das beste Fokus-Ziel: den eindeutigen Namen, sonst die
// Pane-ID (herdr akzeptiert beides als Agent-Target).
func (a Agent) Target() string {
	if strings.TrimSpace(a.Name) != "" {
		return a.Name
	}
	return a.PaneID
}

// Pane spiegelt einen Eintrag aus `herdr pane list` (.result.panes[]).
type Pane struct {
	ID          string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	CWD         string `json:"cwd"`
	AgentName   string `json:"agent"`
	Focused     bool   `json:"focused"`
}

// Client ist die fixe herdr-Schnittstelle (CONTRACTS §6).
type Client interface {
	AgentList(ctx context.Context) ([]Agent, error)
	AgentFocus(ctx context.Context, name string) error
	AgentStart(ctx context.Context, name, kind, paneID string, args []string) error
	PaneList(ctx context.Context) ([]Pane, error)
	PaneSplit(ctx context.Context, paneID, direction string) (string, error)
}

// Runner führt einen herdr-Befehl aus. Er ist eine Naht für Tests.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecClient ist die exec-basierte herdr-Implementierung.
type ExecClient struct {
	Path       string
	RetryCount int
	RetryDelay time.Duration
	Run        Runner
}

// New erstellt einen ExecClient. retryCount/retryDelay steuern den
// Wiederholungsversuch bei fehlendem herdr-Server.
func New(path string, retryCount int, retryDelay time.Duration) *ExecClient {
	if strings.TrimSpace(path) == "" {
		path = "herdr"
	}
	if retryCount < 0 {
		retryCount = 0
	}
	return &ExecClient{Path: path, RetryCount: retryCount, RetryDelay: retryDelay}
}

func (c *ExecClient) runner() Runner {
	if c.Run != nil {
		return c.Run
	}
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		platform.HideConsole(cmd)
		var out, errb bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errb
		if err := cmd.Run(); err != nil {
			return out.Bytes(), fmt.Errorf("%s %s: %w: %s",
				name, strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
		}
		return out.Bytes(), nil
	}
}

func (c *ExecClient) exec(ctx context.Context, args ...string) ([]byte, error) {
	run := c.runner()
	var out []byte
	var err error
	for attempt := 0; ; attempt++ {
		out, err = run(ctx, c.Path, args...)
		if err == nil {
			return out, nil
		}
		if attempt >= c.RetryCount || !shouldRetry(err) {
			return out, err
		}
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(c.RetryDelay):
		}
	}
}

// shouldRetry erkennt einen fehlenden/nicht erreichbaren herdr-Server.
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"socket", "connect", "connection", "server", "refused",
		"unavailable", "timed out", "timeout", "broken pipe", "no such host",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// AgentList ruft `herdr agent list` auf.
func (c *ExecClient) AgentList(ctx context.Context) ([]Agent, error) {
	out, err := c.exec(ctx, "agent", "list")
	if err != nil {
		return nil, err
	}
	return ParseAgentList(out)
}

// AgentFocus ruft `herdr agent focus <name>` auf.
func (c *ExecClient) AgentFocus(ctx context.Context, name string) error {
	_, err := c.exec(ctx, "agent", "focus", name)
	return err
}

// AgentStart ruft `herdr agent start <name> --kind <kind> --pane <id> -- <args>` auf.
func (c *ExecClient) AgentStart(ctx context.Context, name, kind, paneID string, args []string) error {
	argv := []string{"agent", "start", name, "--kind", kind, "--pane", paneID}
	if len(args) > 0 {
		argv = append(argv, "--")
		argv = append(argv, args...)
	}
	_, err := c.exec(ctx, argv...)
	return err
}

// PaneList ruft `herdr pane list` auf.
func (c *ExecClient) PaneList(ctx context.Context) ([]Pane, error) {
	out, err := c.exec(ctx, "pane", "list")
	if err != nil {
		return nil, err
	}
	return ParsePaneList(out)
}

// PaneSplit ruft `herdr pane split <pane> --direction <dir> --no-focus` auf.
func (c *ExecClient) PaneSplit(ctx context.Context, paneID, direction string) (string, error) {
	if strings.TrimSpace(direction) == "" {
		direction = "right"
	}
	out, err := c.exec(ctx, "pane", "split", paneID, "--direction", direction, "--no-focus")
	if err != nil {
		return "", err
	}
	return ParsePaneSplit(out)
}

// FindAgent findet einen Agenten primär über den Namen, sonst über
// Label/Kind (Agenten ohne eindeutigen Namen, z. B. der interaktiv gestartete
// opencode, haben nur das Label "opencode").
func FindAgent(agents []Agent, name, kind string) (Agent, bool) {
	if strings.TrimSpace(name) != "" {
		for _, a := range agents {
			if a.Name == name {
				return a, true
			}
		}
	}
	if strings.TrimSpace(kind) != "" {
		for _, a := range agents {
			if a.Label == kind || a.Kind == kind {
				return a, true
			}
		}
	}
	return Agent{}, false
}

// ParseAgentList parst `.result.agents[]` (mit Array-Fallback).
func ParseAgentList(data []byte) ([]Agent, error) {
	var env struct {
		Result struct {
			Agents []Agent `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &env); err == nil && env.Result.Agents != nil {
		return env.Result.Agents, nil
	}
	var arr []Agent
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("herdr agent list: unerwartetes JSON: %s", strings.TrimSpace(string(data)))
}

// ParsePaneList parst `.result.panes[]` (mit Array-Fallback).
func ParsePaneList(data []byte) ([]Pane, error) {
	var env struct {
		Result struct {
			Panes []Pane `json:"panes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &env); err == nil && env.Result.Panes != nil {
		return env.Result.Panes, nil
	}
	var arr []Pane
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("herdr pane list: unerwartetes JSON: %s", strings.TrimSpace(string(data)))
}

// ParsePaneSplit parst `.result.pane.pane_id`.
func ParsePaneSplit(data []byte) (string, error) {
	var env struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("herdr pane split: %w", err)
	}
	if strings.TrimSpace(env.Result.Pane.PaneID) == "" {
		return "", fmt.Errorf("herdr pane split: keine pane_id in Antwort: %s", strings.TrimSpace(string(data)))
	}
	return env.Result.Pane.PaneID, nil
}
