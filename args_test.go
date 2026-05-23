package main

import (
	"reflect"
	"testing"
)

func TestFlagsFirstKeepsInputFirstUX(t *testing.T) {
	got := flagsFirst(
		[]string{"input.MP4", "--channel", "right", "--fps", "23.976", "--output", "out.mov", "--json"},
		map[string]bool{"channel": true, "fps": true, "output": true},
	)
	want := []string{"--channel", "right", "--fps", "23.976", "--output", "out.mov", "--json", "input.MP4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flagsFirst() = %#v, want %#v", got, want)
	}
}

func TestFlagsFirstSupportsEqualsSyntax(t *testing.T) {
	got := flagsFirst(
		[]string{"input.MP4", "--channel=right", "--fps=23.976", "--output=out.mov"},
		map[string]bool{"channel": true, "fps": true, "output": true},
	)
	want := []string{"--channel=right", "--fps=23.976", "--output=out.mov", "input.MP4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flagsFirst() = %#v, want %#v", got, want)
	}
}

func TestFlagsFirstFixShortcut(t *testing.T) {
	got := flagsFirst(
		[]string{"input.MP4", "--fps", "29.97", "--output", "out.mov"},
		map[string]bool{"fps": true, "output": true},
	)
	want := []string{"--fps", "29.97", "--output", "out.mov", "input.MP4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flagsFirst() = %#v, want %#v", got, want)
	}
}

func TestFlagsFirstBoolFlagAfterInput(t *testing.T) {
	got := flagsFirst(
		[]string{"input.mov", "--json"},
		map[string]bool{},
	)
	want := []string{"--json", "input.mov"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flagsFirst() = %#v, want %#v", got, want)
	}
}
