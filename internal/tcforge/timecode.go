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
	}
	channel = strings.TrimPrefix(channel, "channel")
	channel = strings.TrimPrefix(channel, "ch")
	channel = strings.TrimSpace(channel)
	n, err := strconv.Atoi(channel)
	if err != nil || n < 1 {
		return "", "", fmt.Errorf("channel must be auto, left, right, or a channel number")
	}
	if n == 1 {
		return "left", "c0", nil
	}
	if n == 2 {
		return "right", "c1", nil
	}
	return strconv.Itoa(n), fmt.Sprintf("c%d", n-1), nil
}

func channelNumber(channel string) int {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "left":
		return 1
	case "right":
		return 2
	case "auto", "":
		return 0
	}
	n, _ := strconv.Atoi(channel)
	return n
}

func displayChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "":
		return ""
	case "left":
		return "Left channel"
	case "right":
		return "Right channel"
	default:
		return "Channel " + channel
	}
}

func DisplayChannel(channel string) string {
	return displayChannel(channel)
}

type ltcChannelCandidate struct {
	channel    string
	audioMap   string
	panChannel string
	file       string
}

func ltcChannelCandidates(probe ProbeInfo) []ltcChannelCandidate {
	var candidates []ltcChannelCandidate
	globalChannel := 0
	audioStream := 0
	for _, stream := range probe.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		channels := stream.Channels
		if channels < 1 {
			channels = 1
		}
		for streamChannel := 1; streamChannel <= channels; streamChannel++ {
			globalChannel++
			channel := strconv.Itoa(globalChannel)
			switch globalChannel {
			case 1:
				channel = "left"
			case 2:
				channel = "right"
			}
			candidates = append(candidates, ltcChannelCandidate{
				channel:    channel,
				audioMap:   fmt.Sprintf("0:a:%d", audioStream),
				panChannel: fmt.Sprintf("c%d", streamChannel-1),
				file:       fmt.Sprintf("channel-%d-ltc.wav", globalChannel),
			})
		}
		audioStream++
	}
	return candidates
}

func audioChannelCount(probe ProbeInfo) int {
	total := 0
	for _, stream := range probe.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		if stream.Channels < 1 {
			total++
			continue
		}
		total += stream.Channels
	}
	return total
}

func findLTCChannelCandidate(probe ProbeInfo, channel string) (ltcChannelCandidate, bool) {
	requested := channelNumber(channel)
	if requested < 1 {
		return ltcChannelCandidate{}, false
	}
	for i, candidate := range ltcChannelCandidates(probe) {
		if i+1 == requested || strings.EqualFold(candidate.channel, channel) {
			return candidate, true
		}
	}
	return ltcChannelCandidate{}, false
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
