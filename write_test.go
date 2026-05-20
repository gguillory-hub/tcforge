package main

import (
	"reflect"
	"testing"
)

func TestBuildExtractCommand(t *testing.T) {
	got := buildExtractCommand("in.MP4", "ltc.wav", "c1")
	want := CommandSummary{
		Program: "ffmpeg",
		Args: []string{
			"-y",
			"-i", "in.MP4",
			"-vn",
			"-map", "0:a:0",
			"-af", "pan=mono|c0=c1",
			"-ac", "1",
			"-ar", "48000",
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
		"-write_tmcd", "on",
		"out.mov",
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("buildWriteCommand().Args = %#v, want %#v", got.Args, wantArgs)
	}
}
