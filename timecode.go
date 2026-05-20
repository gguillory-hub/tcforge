package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var timecodePattern = regexp.MustCompile(`\b([0-2]\d):([0-5]\d):([0-5]\d)([:;])([0-5]\d)\b`)

func parseLTCStart(output string) (string, error) {
	match := timecodePattern.FindStringSubmatch(output)
	if match == nil {
		return "", fmt.Errorf("no SMPTE timecode found in ltcdump output")
	}
	return match[0], nil
}

func validateFPS(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("fps is required")
	}
	switch value {
	case "23.976", "24", "25", "29.97", "30", "47.952", "48", "50", "59.94", "60":
		return value, nil
	default:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil || f <= 0 {
			return "", fmt.Errorf("invalid fps %q", raw)
		}
		return value, nil
	}
}

func normalizeChannel(raw string) (string, string, error) {
	channel := strings.ToLower(strings.TrimSpace(raw))
	switch channel {
	case "left", "l", "1":
		return "left", "c0", nil
	case "right", "r", "2":
		return "right", "c1", nil
	default:
		return "", "", fmt.Errorf("channel must be left, right, 1, or 2")
	}
}
