package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
