package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultFixedOutput(t *testing.T) {
	tests := map[string]string{
		"C1315.MP4":          "C1315_tcforge.mov",
		"clips/C1315.MP4":    "clips/C1315_tcforge.mov",
		"clip.with.dots.mp4": "clip.with.dots_tcforge.mov",
		"C1315":              "C1315_tcforge.mov",
	}
	for input, want := range tests {
		if got := defaultFixedOutput(input); got != want {
			t.Fatalf("defaultFixedOutput(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOutputForFixInput(t *testing.T) {
	if got := outputForFixInput("C1315.MP4", "custom.mov", ""); got != "custom.mov" {
		t.Fatalf("outputForFixInput explicit output = %q", got)
	}
	if got := outputForFixInput("clips/C1315.MP4", "", "synced"); got != `synced\C1315_tcforge.mov` && got != "synced/C1315_tcforge.mov" {
		t.Fatalf("outputForFixInput output-dir = %q", got)
	}
	if got := outputForFixInput("clips/C1315.MP4", "", ""); got != "clips/C1315_tcforge.mov" {
		t.Fatalf("outputForFixInput default = %q", got)
	}
}

func TestOutputForFixJob(t *testing.T) {
	job := FixJob{Input: "in.MP4", Output: "explicit.mov"}
	if got := outputForFixJob(job, "", "out"); got != "explicit.mov" {
		t.Fatalf("outputForFixJob() = %q", got)
	}
}

func TestFailedWriteResult(t *testing.T) {
	err := appError("example", "Something failed.", "Try something else.", nil)
	got := failedWriteResult("in.MP4", "out.mov", "auto", err)
	if got.Status != "failed" || got.ErrorCode != "example" || got.Suggestion == "" {
		t.Fatalf("failedWriteResult() = %#v", got)
	}
}

func TestDuplicateOutputAppError(t *testing.T) {
	err := appError("duplicate_output", "duplicate", "pick a different output", nil)
	got := failedWriteResult("in.MP4", "out.mov", "auto", err)
	if got.ErrorCode != "duplicate_output" {
		t.Fatalf("ErrorCode = %q", got.ErrorCode)
	}
}

func TestLoadFixManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	data := `[{"input":"a.MP4","output":"a.mov","fps":"29.97","channel":"right","clean":true}]`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	jobs, err := loadFixManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Input != "a.MP4" || jobs[0].Channel != "right" || jobs[0].Clean == nil || !*jobs[0].Clean {
		t.Fatalf("loadFixManifest() = %#v", jobs)
	}
}

func TestEffectiveClean(t *testing.T) {
	clean := true
	preserve := true
	if !effectiveClean(FixJob{Clean: &clean}, true) {
		t.Fatal("clean override should win")
	}
	if effectiveClean(FixJob{Preserve: &preserve}, false) {
		t.Fatal("preserve job should disable clean")
	}
	if effectiveClean(FixJob{}, true) {
		t.Fatal("preserve flag should disable clean")
	}
}

func TestCanonicalFPS(t *testing.T) {
	tests := map[string]string{
		"30000/1001": "29.97",
		"24000/1001": "23.976",
		"25/1":       "25",
		"60":         "60",
	}
	for input, want := range tests {
		if got := canonicalFPS(input); got != want {
			t.Fatalf("canonicalFPS(%q) = %q, want %q", input, got, want)
		}
	}
}
