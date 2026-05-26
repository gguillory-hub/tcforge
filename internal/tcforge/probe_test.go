package tcforge

import "testing"

func TestSummarizeProbe(t *testing.T) {
	probe := ProbeInfo{
		Format: FormatInfo{Duration: "10.0", Tags: map[string]string{"timecode": "01:00:00:00"}},
		Streams: []StreamInfo{
			{Index: 0, CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080, AvgFrameRate: "30000/1001", Tags: map[string]string{"timecode": "01:00:00:00"}},
			{Index: 1, CodecType: "audio", CodecName: "pcm_s16be", Channels: 2, SampleRate: "48000"},
			{Index: 2, CodecType: "data", CodecName: "unknown", Tags: map[string]string{"handler_name": "Timed Metadata Media Handler", "timecode": "02:00:00:00"}},
		},
	}

	got := summarizeProbe(probe)
	if got.Duration != "10.0" {
		t.Fatalf("Duration = %q, want 10.0", got.Duration)
	}
	if len(got.ExistingTimecodes) != 3 {
		t.Fatalf("ExistingTimecodes length = %d, want 3", len(got.ExistingTimecodes))
	}
	if got.Video[0].Resolution != "1920x1080" {
		t.Fatalf("Resolution = %q, want 1920x1080", got.Video[0].Resolution)
	}
	if got.Video[0].FPS != "30000/1001" {
		t.Fatalf("FPS = %q, want 30000/1001", got.Video[0].FPS)
	}
	if got.Audio[0].Channels != 2 {
		t.Fatalf("Audio channels = %d, want 2", got.Audio[0].Channels)
	}
	if got.Data[0].Handler != "Timed Metadata Media Handler" {
		t.Fatalf("Data handler = %q", got.Data[0].Handler)
	}
}

func TestStreamFPSFallsBackToRFrameRate(t *testing.T) {
	got := streamFPS(StreamInfo{AvgFrameRate: "0/0", RFrameRate: "24/1"})
	if got != "24/1" {
		t.Fatalf("streamFPS() = %q, want 24/1", got)
	}
}

func TestSelectedScore(t *testing.T) {
	scan := LTCScanResult{
		SelectedChannel: "right",
		Channels: []LTCScanChannel{
			{Channel: "left", Score: 1},
			{Channel: "right", Score: 20},
		},
	}
	if got := selectedScore(scan); got != 20 {
		t.Fatalf("selectedScore() = %d, want 20", got)
	}
}

func TestTimecodeMismatch(t *testing.T) {
	existing := []TimecodeReference{{Location: "stream #2", Value: "22:02:07:25"}}
	got := timecodeMismatch(existing, "00:05:22:22")
	if got == "" {
		t.Fatal("expected mismatch warning")
	}
}

func TestTimecodeMismatchIgnoresSeparatorOnlyDifference(t *testing.T) {
	existing := []TimecodeReference{{Location: "format", Value: "00:05:22;22"}}
	got := timecodeMismatch(existing, "00:05:22:22")
	if got != "" {
		t.Fatalf("timecodeMismatch() = %q, want empty", got)
	}
}
