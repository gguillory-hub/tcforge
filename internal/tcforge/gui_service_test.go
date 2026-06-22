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

func TestHumanSummaryFormatsMedia(t *testing.T) {
	scan := ClipScan{
		ClipProbe: ClipProbe{
			Output:      filepath.Join("clips", "C1315_tcforge.mov"),
			InferredFPS: "29.97",
			Summary: ProbeSummary{
				Video: []VideoSummary{{Codec: "h264", Resolution: "3840x2160", FPS: "30000/1001"}},
				Audio: []AudioSummary{{Channels: 2, SampleRate: "48000"}},
			},
		},
		LTCScan: &LTCScanResult{SelectedChannel: "left", SelectedTimecode: "00:05:22:22"},
	}

	got := HumanSummary(scan)
	if got.Video != "4K UHD, 29.97 fps, H.264" {
		t.Fatalf("Video = %q", got.Video)
	}
	if got.Audio != "2 channels, 48 kHz" {
		t.Fatalf("Audio = %q", got.Audio)
	}
	if got.DetectedLTC != "Left channel" {
		t.Fatalf("DetectedLTC = %q", got.DetectedLTC)
	}
	if got.StartTimecode != "00:05:22:22" {
		t.Fatalf("StartTimecode = %q", got.StartTimecode)
	}
	if got.Output != "C1315_tcforge.mov" {
		t.Fatalf("Output = %q", got.Output)
	}
}

func TestAudioDisplaySummarizesMultipleMonoStreams(t *testing.T) {
	got := audioDisplay([]AudioSummary{
		{Channels: 1, SampleRate: "48000"},
		{Channels: 1, SampleRate: "48000"},
		{Channels: 1, SampleRate: "48000"},
		{Channels: 1, SampleRate: "48000"},
	})
	want := "4 mono audio streams, 4 channels, 48 kHz"
	if got != want {
		t.Fatalf("audioDisplay() = %q, want %q", got, want)
	}
}

func TestClassifyWriteResult(t *testing.T) {
	if got := ClassifyWriteResult(WriteResult{Status: "ok"}, nil); got != GUIStatusFixed {
		t.Fatalf("ClassifyWriteResult(ok) = %q", got)
	}
	if got := ClassifyWriteResult(WriteResult{ErrorCode: "ltc_not_found"}, assertErr{}); got != GUIStatusNoAudioLTCFound {
		t.Fatalf("ClassifyWriteResult(ltc_not_found) = %q", got)
	}
	if got := ClassifyWriteResult(WriteResult{ErrorCode: "output_exists"}, assertErr{}); got != GUIStatusNeedsAttention {
		t.Fatalf("ClassifyWriteResult(output_exists) = %q", got)
	}
}

func TestHasTCForgeMetadata(t *testing.T) {
	tagged := ProbeInfo{Format: FormatInfo{Tags: map[string]string{"tcforge": "1"}}}
	if !hasTCForgeMetadata(tagged) {
		t.Fatal("expected tcforge metadata tag")
	}
	generic := ProbeInfo{Format: FormatInfo{Tags: map[string]string{"timecode": "01:00:00:00"}}}
	if hasTCForgeMetadata(generic) {
		t.Fatal("generic timecode should not be classified as tcforge metadata")
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "test error" }

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
}
