package main

import "testing"

func TestAnalyzeVerificationCleanTimecodeFile(t *testing.T) {
	probe := ProbeInfo{
		Streams: []StreamInfo{
			{Index: 0, CodecType: "video", AvgFrameRate: "30000/1001", Tags: map[string]string{"timecode": "00:05:22:22"}},
			{Index: 1, CodecType: "data", Tags: map[string]string{"timecode": "00:05:22:22"}},
		},
	}
	got := analyzeVerification("clip.mov", probe)
	if got.Status != "ok" {
		t.Fatalf("Status = %q, want ok; checks = %#v", got.Status, got.Checks)
	}
	if got.Timecode != "00:05:22:22" {
		t.Fatalf("Timecode = %q", got.Timecode)
	}
	if got.FPS != "29.97" {
		t.Fatalf("FPS = %q, want 29.97", got.FPS)
	}
}

func TestAnalyzeVerificationWarnsWhenAudioPresent(t *testing.T) {
	probe := ProbeInfo{
		Streams: []StreamInfo{
			{Index: 0, CodecType: "video", AvgFrameRate: "24000/1001", Tags: map[string]string{"timecode": "01:00:00:00"}},
			{Index: 1, CodecType: "audio", Channels: 2},
			{Index: 2, CodecType: "data", Tags: map[string]string{"timecode": "01:00:00:00"}},
		},
	}
	got := analyzeVerification("clip.mov", probe)
	if got.Status != "warning" {
		t.Fatalf("Status = %q, want warning; checks = %#v", got.Status, got.Checks)
	}
	if !hasVerifyCheck(got.Checks, "audio_removed", "warn") {
		t.Fatalf("expected audio_removed warning, checks = %#v", got.Checks)
	}
}

func TestAnalyzeVerificationFailsWithoutTmcdTrack(t *testing.T) {
	probe := ProbeInfo{
		Streams: []StreamInfo{
			{Index: 0, CodecType: "video", AvgFrameRate: "25/1", Tags: map[string]string{"timecode": "01:00:00:00"}},
		},
	}
	got := analyzeVerification("clip.mov", probe)
	if got.Status != "failed" {
		t.Fatalf("Status = %q, want failed; checks = %#v", got.Status, got.Checks)
	}
	if !hasVerifyCheck(got.Checks, "tmcd_track", "failed") {
		t.Fatalf("expected tmcd_track failure, checks = %#v", got.Checks)
	}
}

func TestVerificationStatus(t *testing.T) {
	if got := verificationStatus([]VerifyCheck{{Status: "ok"}, {Status: "warn"}}); got != "warning" {
		t.Fatalf("verificationStatus warning = %q", got)
	}
	if got := verificationStatus([]VerifyCheck{{Status: "ok"}, {Status: "failed"}}); got != "failed" {
		t.Fatalf("verificationStatus failed = %q", got)
	}
}

func hasVerifyCheck(checks []VerifyCheck, name, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}
