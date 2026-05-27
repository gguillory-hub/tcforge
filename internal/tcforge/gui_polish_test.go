package tcforge

import "testing"

func TestGUIStatusTone(t *testing.T) {
	tests := map[string]string{
		GUIStatusFixed:              GUIStatusToneSuccess,
		GUIStatusNeedsAttention:     GUIStatusToneWarning,
		GUIStatusAlreadyHasTimecode: GUIStatusToneWarning,
		GUIStatusFailed:             GUIStatusToneError,
		GUIStatusNoAudioLTCFound:    GUIStatusToneError,
		GUIStatusScanning:           GUIStatusToneActive,
		GUIStatusProcessing:         GUIStatusToneActive,
		GUIStatusReady:              GUIStatusToneNeutral,
		GUIStatusAlreadyProcessed:   GUIStatusToneNeutral,
	}
	for status, want := range tests {
		if got := GUIStatusTone(status); got != want {
			t.Fatalf("GUIStatusTone(%q) = %q, want %q", status, got, want)
		}
	}
}
