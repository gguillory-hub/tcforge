package tcforge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var requiredTools = []string{"ffmpeg", "ffprobe", "ltcdump"}
var executablePath = os.Executable

const (
	defaultCommandTimeout = 30 * time.Minute
	ffmpegCommandTimeout  = 12 * time.Hour
)

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
	ctx, cancel := context.WithTimeout(ctx, commandTimeout(program))
	defer cancel()

	resolvedProgram := program
	if resolved, err := resolveTool(program); err == nil {
		resolvedProgram = resolved
	}
	cmd := exec.CommandContext(ctx, resolvedProgram, args...)
	configureCommand(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), commandTimeoutError(program, ctx.Err())
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

func runCommandWithProgress(ctx context.Context, program string, args []string, duration string, progress func(float64)) (string, string, error) {
	if program != "ffmpeg" || progress == nil || duration == "" {
		return runCommand(ctx, program, args...)
	}
	seconds, err := strconv.ParseFloat(duration, 64)
	if err != nil || seconds <= 0 {
		return runCommand(ctx, program, args...)
	}

	progressArgs := append([]string{}, args...)
	progressArgs = append([]string{"-progress", "pipe:1", "-nostats"}, progressArgs...)
	ctx, cancel := context.WithTimeout(ctx, commandTimeout(program))
	defer cancel()

	resolvedProgram := program
	if resolved, err := resolveTool(program); err == nil {
		resolvedProgram = resolved
	}
	cmd := exec.CommandContext(ctx, resolvedProgram, progressArgs...)
	configureCommand(cmd)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", stderr.String(), err
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		readFFmpegProgress(stdoutPipe, &stdout, seconds, progress)
	}()
	err = cmd.Wait()
	<-done
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), commandTimeoutError(program, ctx.Err())
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		summary := fmt.Sprintf("%s failed.", program)
		if detail != "" {
			summary += "\n" + detail
		}
		return stdout.String(), stderr.String(), appError("external_tool_failed", summary, "Run again with --verbose and check the external command output above.", err)
	}
	progress(1)
	return stdout.String(), stderr.String(), nil
}

func commandTimeout(program string) time.Duration {
	if strings.EqualFold(filepath.Base(program), "ffmpeg") || strings.EqualFold(filepath.Base(program), "ffmpeg.exe") {
		return ffmpegCommandTimeout
	}
	return defaultCommandTimeout
}

func commandTimeoutError(program string, err error) error {
	if errors.Is(err, context.Canceled) {
		return appError(
			"command_canceled",
			fmt.Sprintf("%s was canceled.", program),
			"The job was canceled. Any partial output file from the active write was removed.",
			err,
		)
	}
	return appError(
		"command_timeout",
		fmt.Sprintf("%s timed out.", program),
		"Large camera files can take a long time on external or network storage. Confirm the drive is responsive and try again; if this repeats, report the clip duration, file size, and storage type.",
		err,
	)
}

func readFFmpegProgress(reader io.Reader, stdout *bytes.Buffer, durationSeconds float64, progress func(float64)) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		stdout.WriteString(line + "\n")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_ms":
			ms, err := strconv.ParseFloat(value, 64)
			if err == nil && durationSeconds > 0 {
				percent := (ms / 1000000) / durationSeconds
				if percent < 0 {
					percent = 0
				}
				if percent > 1 {
					percent = 1
				}
				progress(percent)
			}
		case "progress":
			if value == "end" {
				progress(1)
			}
		}
	}
}

func resolveTool(name string) (string, error) {
	for _, envName := range toolEnvNames(name) {
		if override := os.Getenv(envName); override != "" {
			if fileExists(override) {
				return override, nil
			}
			return "", fmt.Errorf("%s points to missing file: %s", envName, override)
		}
	}
	if path, err := bundledToolPath(name); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
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

func bundledToolPath(name string) (string, error) {
	exe, err := executablePath()
	if err != nil {
		return "", err
	}
	toolsDir := filepath.Join(filepath.Dir(exe), "tools")
	for _, candidateName := range toolExecutableNames(name) {
		candidate := filepath.Join(toolsDir, candidateName)
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func toolExecutableNames(name string) []string {
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		return []string{name}
	}
	return []string{name, name + ".exe"}
}

func toolEnvNames(name string) []string {
	upperName := strings.ToUpper(name)
	return []string{"TCFORGE_" + upperName, "LTC2META_" + upperName}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
