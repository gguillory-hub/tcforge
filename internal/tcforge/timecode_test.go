package tcforge

import "testing"

func TestParseLTCStart(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "colon timecode",
			output: "LTC frame: 01:02:03:04 ok",
			want:   "01:02:03:04",
		},
		{
			name:   "drop frame timecode",
			output: "position 00:59:59;29",
			want:   "00:59:59;29",
		},
		{
			name:   "first timecode wins",
			output: "start 10:00:00:00 later 10:00:00:01",
			want:   "10:00:00:00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLTCStart(tt.output)
			if err != nil {
				t.Fatalf("parseLTCStart() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseLTCStart() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLTCStartMissing(t *testing.T) {
	if _, err := parseLTCStart("no useful decode here"); err == nil {
		t.Fatal("parseLTCStart() expected error")
	}
}

func TestFindTimecodes(t *testing.T) {
	got := findTimecodes("start 00:05:22:22 later 00:05:22:23")
	want := []string{"00:05:22:22", "00:05:22:23"}
	if len(got) != len(want) {
		t.Fatalf("findTimecodes() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("findTimecodes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateFPS(t *testing.T) {
	for _, fps := range []string{"23.976", "24", "29.97", "59.94", "120"} {
		got, err := validateFPS(fps)
		if err != nil {
			t.Fatalf("validateFPS(%q) error = %v", fps, err)
		}
		if got != fps {
			t.Fatalf("validateFPS(%q) = %q", fps, got)
		}
	}
}

func TestValidateFPSRejectsInvalid(t *testing.T) {
	for _, fps := range []string{"", "zero", "-1"} {
		if _, err := validateFPS(fps); err == nil {
			t.Fatalf("validateFPS(%q) expected error", fps)
		}
	}
}

func TestNormalizeChannel(t *testing.T) {
	tests := map[string]string{
		"auto":  "",
		"":      "",
		"left":  "c0",
		"1":     "c0",
		"L":     "c0",
		"right": "c1",
		"2":     "c1",
		"R":     "c1",
	}
	for input, wantPan := range tests {
		_, gotPan, err := normalizeChannel(input)
		if err != nil {
			t.Fatalf("normalizeChannel(%q) error = %v", input, err)
		}
		if gotPan != wantPan {
			t.Fatalf("normalizeChannel(%q) pan = %q, want %q", input, gotPan, wantPan)
		}
	}
}

func TestLTCDecodeScore(t *testing.T) {
	clean := "#User bits  Timecode\n20210920   00:05:22:22 | 1390 2993\n20210920   00:05:22:23 | 2994 4595\n"
	noisy := "#User bits  Timecode\n#DISCONTINUITY\n5466d778   25:44:43.11 | 233311 233525\n"

	if got := ltcDecodeScore(clean); got <= ltcDecodeScore(noisy) {
		t.Fatalf("clean score = %d, noisy score = %d; clean should win", got, ltcDecodeScore(noisy))
	}
	if got := ltcDecodeScore("nothing here"); got != 0 {
		t.Fatalf("ltcDecodeScore(empty) = %d, want 0", got)
	}
}

func TestPlausibleTimecode(t *testing.T) {
	if !plausibleTimecode("00:05:22:22") {
		t.Fatal("expected normal timecode to be plausible")
	}
	if plausibleTimecode("25:44:43.11") {
		t.Fatal("expected dotted/noisy timecode to be implausible")
	}
	if plausibleTimecode("37:73:75:34") {
		t.Fatal("expected invalid timecode to be implausible")
	}
}

func TestLTCDumpFPS(t *testing.T) {
	tests := map[string]string{
		"23.976": "24000/1001",
		"29.97":  "30000/1001",
		"59.94":  "60000/1001",
		"24":     "24",
	}
	for input, want := range tests {
		if got := ltcDumpFPS(input); got != want {
			t.Fatalf("ltcDumpFPS(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFPSMatches(t *testing.T) {
	if !fpsMatches("30000/1001", "29.97") {
		t.Fatal("expected 30000/1001 to match 29.97")
	}
	if fpsMatches("25/1", "29.97") {
		t.Fatal("expected 25/1 not to match 29.97")
	}
}

func TestParseFPS(t *testing.T) {
	got, err := parseFPS("30000/1001")
	if err != nil {
		t.Fatal(err)
	}
	if got < 29.96 || got > 29.98 {
		t.Fatalf("parseFPS() = %f", got)
	}
}
