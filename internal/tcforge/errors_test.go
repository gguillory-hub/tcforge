package tcforge

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapProbeErrorIncludesExternalToolFailureDetails(t *testing.T) {
	cause := appError("external_tool_failed", "ffprobe failed.\ndyld: Library not loaded", "check ffprobe", errors.New("exit 1"))

	got := wrapProbeError("clip.MP4", cause).Error()
	for _, want := range []string{"Could not read clip.MP4", "ffprobe failed.", "dyld: Library not loaded", "xattr -dr com.apple.quarantine"} {
		if !strings.Contains(got, want) {
			t.Fatalf("wrapProbeError() = %q, want it to contain %q", got, want)
		}
	}
}
