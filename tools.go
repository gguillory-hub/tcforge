package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var requiredTools = []string{"ffmpeg", "ffprobe", "ltcdump"}

type ToolStatus struct {
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"`
	Found bool   `json:"found"`
}

type CommandSummary struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
}

func checkRequiredTools() ([]ToolStatus, error) {
	statuses := make([]ToolStatus, 0, len(requiredTools))
	var missing []string
	for _, name := range requiredTools {
		path, err := exec.LookPath(name)
		status := ToolStatus{Name: name, Path: path, Found: err == nil}
		statuses = append(statuses, status)
		if err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return statuses, fmt.Errorf("missing required tools on PATH: %s", strings.Join(missing, ", "))
	}
	return statuses, nil
}

func runCommand(ctx context.Context, program string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, program, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), ctx.Err()
	}
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("%s failed: %w\n%s", program, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), stderr.String(), nil
}
