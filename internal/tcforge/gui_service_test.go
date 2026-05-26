package tcforge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultOutputMatchesFixBehavior(t *testing.T) {
	input := filepath.Join("clips", "C1315.MP4")
	got := DefaultOutput(input, "")
	want := filepath.Join("clips", "C1315_tcforge.mov")
	if got != want {
		t.Fatalf("DefaultOutput() = %q, want %q", got, want)
	}
}

func TestDefaultOutputUsesOutputDir(t *testing.T) {
	input := filepath.Join("clips", "C1315.MP4")
	got := DefaultOutput(input, "out")
	want := filepath.Join("out", "C1315_tcforge.mov")
	if got != want {
		t.Fatalf("DefaultOutput() = %q, want %q", got, want)
	}
}

func TestListMediaFilesFiltersSupportedExtensions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.MP4", "b.mov", "c.mxf", "d.txt"} {
		touch(t, filepath.Join(dir, name))
	}

	got, err := ListMediaFiles(dir)
	if err != nil {
		t.Fatalf("ListMediaFiles() error = %v", err)
	}
	var bases []string
	for _, path := range got {
		bases = append(bases, filepath.Base(path))
	}
	want := []string{"a.MP4", "b.mov", "c.mxf"}
	if !reflect.DeepEqual(bases, want) {
		t.Fatalf("ListMediaFiles() = %#v, want %#v", bases, want)
	}
}

func TestInferredVideoFPSUsesCanonicalFPS(t *testing.T) {
	probe := ProbeInfo{Streams: []StreamInfo{{CodecType: "video", AvgFrameRate: "30000/1001"}}}
	got := inferredVideoFPS(probe)
	if got != "29.97" {
		t.Fatalf("inferredVideoFPS() = %q, want 29.97", got)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
}
