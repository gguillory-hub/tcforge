package tcforge

import (
	"reflect"
	"testing"
)

func TestBuildExtractCommand(t *testing.T) {
	got := buildExtractCommand("in.MP4", "ltc.wav", "0:a:1", "c0")
	want := CommandSummary{
		Program: "ffmpeg",
		Args: []string{
			"-y",
			"-i", "in.MP4",
			"-vn",
			"-map", "0:a:1",
			"-af", "pan=mono|c0=c0",
			"-ac", "1",
			"-ar", "48000",
			"-t", ltcExtractDuration,
			"ltc.wav",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildExtractCommand() = %#v, want %#v", got, want)
	}
}

func TestBuildWriteCommand(t *testing.T) {
	got := buildWriteCommand(WriteOptions{
		Input:  "in.MP4",
		Output: "out.mov",
	}, "10:00:00:00")
	want := CommandSummary{
		Program: "ffmpeg",
		Args: []string{
			"-n",
			"-i", "in.MP4",
			"-map", "0",
			"-c", "copy",
			"-timecode", "10:00:00:00",
			"-metadata", "timecode=10:00:00:00",
			"-metadata", "tcforge=1",
			"-write_tmcd", "on",
			"out.mov",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildWriteCommand() = %#v, want %#v", got, want)
	}
}

func TestBuildWriteCommandDropLTCAudio(t *testing.T) {
	got := buildWriteCommand(WriteOptions{
		Input:        "in.MP4",
		Output:       "out.mov",
		Overwrite:    true,
		DropLTCAudio: true,
	}, "10:00:00:00")
	wantArgs := []string{
		"-y",
		"-i", "in.MP4",
		"-map", "0",
		"-map", "-0:a:0",
		"-c", "copy",
		"-timecode", "10:00:00:00",
		"-metadata", "timecode=10:00:00:00",
		"-metadata", "tcforge=1",
		"-write_tmcd", "on",
		"out.mov",
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("buildWriteCommand().Args = %#v, want %#v", got.Args, wantArgs)
	}
}

func TestBuildWriteCommandClean(t *testing.T) {
	got := buildWriteCommand(WriteOptions{
		Input:  "in.MP4",
		Output: "out.mov",
		Clean:  true,
	}, "10:00:00:00")
	wantArgs := []string{
		"-n",
		"-i", "in.MP4",
		"-map", "0:v:0",
		"-c", "copy",
		"-timecode", "10:00:00:00",
		"-metadata", "timecode=10:00:00:00",
		"-metadata", "tcforge=1",
		"-write_tmcd", "on",
		"out.mov",
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("buildWriteCommand().Args = %#v, want %#v", got.Args, wantArgs)
	}
}

func TestBuildWriteCommandCleanIgnoresDropLTCAudio(t *testing.T) {
	got := buildWriteCommand(WriteOptions{
		Input:        "in.MP4",
		Output:       "out.mov",
		Clean:        true,
		DropLTCAudio: true,
	}, "10:00:00:00")
	for i, arg := range got.Args {
		if arg == "-map" && i+1 < len(got.Args) && got.Args[i+1] == "-0:a:0" {
			t.Fatal("clean mode should not add a redundant audio removal map")
		}
	}
}

func TestEnsureAudioChannelAllowsFourthChannel(t *testing.T) {
	probe := ProbeInfo{Streams: []StreamInfo{{CodecType: "audio", Channels: 4}}}
	if err := ensureAudioChannel(probe, "4"); err != nil {
		t.Fatalf("ensureAudioChannel() error = %v", err)
	}
}

func TestEnsureAudioChannelRejectsMissingFourthChannel(t *testing.T) {
	probe := ProbeInfo{Streams: []StreamInfo{{CodecType: "audio", Channels: 2}}}
	if err := ensureAudioChannel(probe, "4"); err == nil {
		t.Fatal("expected channel 4 to fail when audio stream only has 2 channels")
	}
}

func TestWriteMode(t *testing.T) {
	if got := writeMode([]CommandSummary{buildWriteCommand(WriteOptions{Input: "in.MP4", Output: "out.mov"}, "10:00:00:00")}); got != "preserve" {
		t.Fatalf("writeMode() = %q, want preserve", got)
	}
	if got := writeMode([]CommandSummary{buildWriteCommand(WriteOptions{Input: "in.MP4", Output: "out.mov", Clean: true}, "10:00:00:00")}); got != "clean" {
		t.Fatalf("writeMode() = %q, want clean", got)
	}
}

func TestEnsureFPSMatch(t *testing.T) {
	probe := ProbeInfo{Streams: []StreamInfo{{CodecType: "video", AvgFrameRate: "30000/1001"}}}
	if err := ensureFPSMatch(probe, "29.97"); err != nil {
		t.Fatalf("ensureFPSMatch() error = %v", err)
	}
	if err := ensureFPSMatch(probe, "25"); err == nil {
		t.Fatal("ensureFPSMatch() expected mismatch error")
	}
}

func TestEnsureVideoStream(t *testing.T) {
	if err := ensureVideoStream(ProbeInfo{Streams: []StreamInfo{{CodecType: "video"}}}); err != nil {
		t.Fatalf("ensureVideoStream() error = %v", err)
	}
	if err := ensureVideoStream(ProbeInfo{Streams: []StreamInfo{{CodecType: "audio"}}}); err == nil {
		t.Fatal("ensureVideoStream() expected error")
	}
}
