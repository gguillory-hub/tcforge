//go:build gui && windows

package main

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"syscall"

	nativedialog "github.com/sqweek/dialog"
)

func openMediaFiles() ([]string, error) {
	const script = `
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.OpenFileDialog
$dialog.Title = 'Add Media Files'
$dialog.Filter = 'Media files (*.mp4;*.mov;*.m4v;*.mxf)|*.mp4;*.mov;*.m4v;*.mxf|All files (*.*)|*.*'
$dialog.Multiselect = $true
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  $dialog.FileNames | ConvertTo-Json -Compress
} else {
  exit 2
}
`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return nil, nativedialog.ErrCancelled
		}
		return nil, err
	}
	var paths []string
	if err := json.Unmarshal(out, &paths); err != nil {
		var single string
		if singleErr := json.Unmarshal(out, &single); singleErr == nil && strings.TrimSpace(single) != "" {
			return []string{single}, nil
		}
		return nil, err
	}
	return paths, nil
}
