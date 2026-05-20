package main

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
