package tcforge

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var timecodePattern = regexp.MustCompile(`\b([0-2]\d):([0-5]\d):([0-5]\d)([:;])([0-5]\d)\b`)

func parseLTCStart(output string) (string, error) {
	timecodes := findTimecodes(output)
	if len(timecodes) == 0 {
		return "", fmt.Errorf("no SMPTE timecode found in ltcdump output")
	}
	return timecodes[0], nil
}

func findTimecodes(output string) []string {
	matches := timecodePattern.FindAllStringSubmatch(output, -1)
	timecodes := make([]string, 0, len(matches))
	for _, match := range matches {
		if match == nil {
			continue
		}
		timecodes = append(timecodes, match[0])
	}
	return timecodes
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

func parseFPS(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "0/0" {
		return 0, fmt.Errorf("empty fps")
	}
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid fps %q", raw)
		}
		num, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		den, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		if den == 0 {
			return 0, fmt.Errorf("invalid fps denominator")
		}
		return num / den, nil
	}
	return strconv.ParseFloat(value, 64)
}

func fpsMatches(a, b string) bool {
	aa, errA := parseFPS(a)
	bb, errB := parseFPS(b)
	if errA != nil || errB != nil {
		return false
	}
	return math.Abs(aa-bb) < 0.01
}

func canonicalFPS(raw string) string {
	fps, err := parseFPS(raw)
	if err != nil || fps <= 0 {
		return ""
	}
	known := []struct {
		value float64
		label string
	}{
		{24000.0 / 1001.0, "23.976"},
		{24, "24"},
		{25, "25"},
		{30000.0 / 1001.0, "29.97"},
		{30, "30"},
		{48000.0 / 1001.0, "47.952"},
		{48, "48"},
		{50, "50"},
		{60000.0 / 1001.0, "59.94"},
		{60, "60"},
	}
	for _, candidate := range known {
		if math.Abs(fps-candidate.value) < 0.01 {
			return candidate.label
		}
	}
	return strings.TrimSpace(raw)
}

func ltcDumpFPS(raw string) string {
	switch strings.TrimSpace(raw) {
	case "23.976":
		return "24000/1001"
	case "29.97":
		return "30000/1001"
	case "47.952":
		return "48000/1001"
	case "59.94":
		return "60000/1001"
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeChannel(raw string) (string, string, error) {
	channel := strings.ToLower(strings.TrimSpace(raw))
	switch channel {
	case "auto", "a", "":
		return "auto", "", nil
	case "left", "l", "1":
		return "left", "c0", nil
	case "right", "r", "2":
		return "right", "c1", nil
	default:
		return "", "", fmt.Errorf("channel must be auto, left, right, 1, or 2")
	}
}

func ltcDecodeScore(output string) int {
	score := 0
	for _, tc := range findTimecodes(output) {
		if plausibleTimecode(tc) {
			score += 10
		} else {
			score++
		}
	}
	score -= strings.Count(output, "#DISCONTINUITY")
	if score < 0 {
		return 0
	}
	return score
}

func plausibleTimecode(tc string) bool {
	match := timecodePattern.FindStringSubmatch(tc)
	if match == nil {
		return false
	}
	frames, err := strconv.Atoi(match[5])
	if err != nil {
		return false
	}
	return frames <= 30
}
