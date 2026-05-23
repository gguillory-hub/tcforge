package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		path, err := resolveTool(name)
		status := ToolStatus{Name: name, Path: path, Found: err == nil}
		statuses = append(statuses, status)
		if err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return statuses, appError(
			"missing_tools",
			fmt.Sprintf("Missing required external tools: %s.", strings.Join(missing, ", ")),
			"Install ffmpeg/ffprobe and ltcdump, then make sure they are available on PATH or through TCFORGE_* environment variables.",
			nil,
		)
	}
	return statuses, nil
}

func runCommand(ctx context.Context, program string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	resolvedProgram := program
	if resolved, err := resolveTool(program); err == nil {
		resolvedProgram = resolved
	}
	cmd := exec.CommandContext(ctx, resolvedProgram, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), appError("command_timeout", fmt.Sprintf("%s timed out.", program), "Try a shorter clip or confirm the external tool is not waiting for input.", ctx.Err())
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		summary := fmt.Sprintf("%s failed.", program)
		if detail != "" {
			summary += "\n" + detail
		}
		return stdout.String(), stderr.String(), appError("external_tool_failed", summary, "Run again with --verbose and check the external command output above.", err)
	}
	return stdout.String(), stderr.String(), nil
}

func resolveTool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, envName := range toolEnvNames(name) {
		if override := os.Getenv(envName); override != "" {
			if fileExists(override) {
				return override, nil
			}
			return "", fmt.Errorf("%s points to missing file: %s", envName, override)
		}
	}
	if name == "ltcdump" {
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			candidate := filepath.Join(userProfile, "tools", "ltcdump", "ltcdump.exe")
			if fileExists(candidate) {
				return candidate, nil
			}
		}
	}
	return "", exec.ErrNotFound
}

func toolEnvNames(name string) []string {
	upperName := strings.ToUpper(name)
	return []string{"TCFORGE_" + upperName, "LTC2META_" + upperName}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
