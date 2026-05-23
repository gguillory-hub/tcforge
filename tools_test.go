package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveToolUsesBundledToolBeforePath(t *testing.T) {
	dir := t.TempDir()
	toolsDir := filepath.Join(dir, "tools")
	if err := os.Mkdir(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "tcforge.exe")
	if err := os.WriteFile(exe, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	bundled := filepath.Join(toolsDir, "ltcfake.exe")
	if err := os.WriteFile(bundled, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	pathDir := t.TempDir()
	pathTool := filepath.Join(pathDir, "ltcfake.exe")
	if err := os.WriteFile(pathTool, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)
	withExecutablePath(t, exe)

	got, err := resolveTool("ltcfake")
	if err != nil {
		t.Fatalf("resolveTool() error = %v", err)
	}
	if got != bundled {
		t.Fatalf("resolveTool() = %q, want bundled %q", got, bundled)
	}
}

func TestResolveToolUsesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(exe, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TCFORGE_LTCFAKE", exe)

	got, err := resolveTool("ltcfake")
	if err != nil {
		t.Fatalf("resolveTool() error = %v", err)
	}
	if got != exe {
		t.Fatalf("resolveTool() = %q, want %q", got, exe)
	}
}

func TestResolveToolEnvOverrideWinsOverBundledTool(t *testing.T) {
	dir := t.TempDir()
	toolsDir := filepath.Join(dir, "tools")
	if err := os.Mkdir(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(dir, "tcforge.exe")
	if err := os.WriteFile(app, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	bundled := filepath.Join(toolsDir, "ltcfake.exe")
	if err := os.WriteFile(bundled, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(t.TempDir(), "override.exe")
	if err := os.WriteFile(override, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	withExecutablePath(t, app)
	t.Setenv("TCFORGE_LTCFAKE", override)

	got, err := resolveTool("ltcfake")
	if err != nil {
		t.Fatalf("resolveTool() error = %v", err)
	}
	if got != override {
		t.Fatalf("resolveTool() = %q, want env override %q", got, override)
	}
}

func TestResolveToolUsesLegacyEnvOverride(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(exe, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LTC2META_LTCFAKE", exe)

	got, err := resolveTool("ltcfake")
	if err != nil {
		t.Fatalf("resolveTool() error = %v", err)
	}
	if got != exe {
		t.Fatalf("resolveTool() = %q, want %q", got, exe)
	}
}

func TestResolveToolRejectsMissingEnvOverride(t *testing.T) {
	t.Setenv("TCFORGE_LTCFAKE", filepath.Join(t.TempDir(), "missing.exe"))

	if _, err := resolveTool("ltcfake"); err == nil {
		t.Fatal("resolveTool() expected error")
	}
}

func TestResolveToolMissingTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	withExecutablePath(t, filepath.Join(t.TempDir(), "tcforge.exe"))

	if _, err := resolveTool("tcforge-definitely-missing-tool"); err == nil || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("resolveTool() error = %v, want missing tool error", err)
	}
}

func TestVersionStringIncludesInjectedVersion(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
	})
	version = "v1.2.3"
	commit = "abc123"
	date = "2026-05-23T12:00:00Z"

	got := versionString()
	for _, want := range []string{"tcforge v1.2.3", "commit=abc123", "built=2026-05-23T12:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("versionString() = %q, want it to contain %q", got, want)
		}
	}
}

func withExecutablePath(t *testing.T, path string) {
	t.Helper()
	old := executablePath
	executablePath = func() (string, error) {
		return path, nil
	}
	t.Cleanup(func() {
		executablePath = old
	})
}
