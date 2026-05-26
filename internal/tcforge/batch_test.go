package tcforge

import "testing"

func TestIsSupportedMedia(t *testing.T) {
	for _, path := range []string{"clip.MP4", "clip.mov", "clip.MXF", "clip.m4v"} {
		if !isSupportedMedia(path) {
			t.Fatalf("isSupportedMedia(%q) = false, want true", path)
		}
	}
}

func TestIsSupportedMediaRejectsOtherFiles(t *testing.T) {
	for _, path := range []string{"notes.txt", "audio.wav", "image.jpg"} {
		if isSupportedMedia(path) {
			t.Fatalf("isSupportedMedia(%q) = true, want false", path)
		}
	}
}
