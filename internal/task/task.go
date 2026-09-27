// Package task registriert/entfernt den Windows-Task
// `Lars-Win-AI\aid` (Trigger "At log on") über PowerShell
// (Register-ScheduledTask), robust per -EncodedCommand.
package task

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf16"
)

const (
	// Name ist der Task-Name.
	Name = "aid"
	// Dir ist der Task-Ordner (TaskPath).
	Dir = `\Lars-Win-AI\`
)

// Register legt den Task idempotent an (-Force).
func Register(ctx context.Context, exe string, args []string) error {
	script := strings.NewReplacer(
		"{{EXE}}", psQuote(exe),
		"{{ARGS}}", psQuote(strings.Join(args, " ")),
		"{{NAME}}", psQuote(Name),
		"{{DIR}}", psQuote(Dir),
	).Replace(registerScript)
	_, err := runPowerShell(ctx, script)
	return err
}

// Unregister entfernt den Task idempotent.
func Unregister(ctx context.Context) error {
	_, err := runPowerShell(ctx, unregisterScript)
	return err
}

// Query liefert (installed, info, err). info fasst State/LastRun/Result zusammen.
func Query(ctx context.Context) (bool, string, error) {
	out, err := runPowerShell(ctx, queryScript)
	info := strings.TrimSpace(out)
	if err != nil {
		return false, info, err
	}
	if info == "" || strings.Contains(info, "NotFound") {
		return false, info, nil
	}
	return true, info, nil
}

const registerScript = `$ErrorActionPreference = 'Stop'
$action = New-ScheduledTaskAction -Execute '{{EXE}}' -Argument '{{ARGS}}'
$trigger = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
$principal = New-ScheduledTaskPrincipal -UserId ("{0}\{1}" -f $env:USERDOMAIN, $env:USERNAME) -LogonType Interactive -RunLevel Highest
Register-ScheduledTask -TaskName '{{NAME}}' -TaskPath '{{DIR}}' -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
Write-Output 'OK'
`

const unregisterScript = `$ErrorActionPreference = 'Stop'
try {
  Unregister-ScheduledTask -TaskName '{{NAME}}' -TaskPath '{{DIR}}' -Confirm:$false -ErrorAction Stop | Out-Null
  Write-Output 'OK'
} catch {
  if ($_.FullyQualifiedErrorId -match 'NotFound' -or $_.CategoryInfo.Reason -eq 'ObjectNotFound') {
    Write-Output 'NOTFOUND'
  } else {
    throw
  }
}
`

const queryScript = `$ErrorActionPreference = 'SilentlyContinue'
$t = Get-ScheduledTask -TaskName '{{NAME}}' -TaskPath '{{DIR}}'
if ($null -eq $t) { Write-Output 'NotFound'; exit 0 }
$i = $t | Get-ScheduledTaskInfo
Write-Output ('State=' + $t.State + ' LastRunTime=' + $i.LastRunTime + ' LastTaskResult=' + $i.LastTaskResult)
`

func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func runPowerShell(ctx context.Context, script string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(utf16LE(script))
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encoded,
	)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

func utf16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[i*2] = byte(v)
		b[i*2+1] = byte(v >> 8)
	}
	return b
}
