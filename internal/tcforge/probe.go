package tcforge

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ProbeInfo struct {
	Streams []StreamInfo `json:"streams"`
	Format  FormatInfo   `json:"format"`
}

type StreamInfo struct {
	Index         int               `json:"index"`
	CodecType     string            `json:"codec_type"`
	CodecName     string            `json:"codec_name,omitempty"`
	Width         int               `json:"width,omitempty"`
	Height        int               `json:"height,omitempty"`
	Channels      int               `json:"channels,omitempty"`
	ChannelLayout string            `json:"channel_layout,omitempty"`
	SampleRate    string            `json:"sample_rate,omitempty"`
	AvgFrameRate  string            `json:"avg_frame_rate,omitempty"`
	RFrameRate    string            `json:"r_frame_rate,omitempty"`
	Duration      string            `json:"duration,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

type FormatInfo struct {
	Filename   string            `json:"filename"`
	Duration   string            `json:"duration,omitempty"`
	FormatName string            `json:"format_name,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type ProbeResult struct {
	Input   string         `json:"input"`
	Tools   []ToolStatus   `json:"tools"`
	Probe   ProbeInfo      `json:"probe"`
	Summary ProbeSummary   `json:"summary"`
	LTCScan *LTCScanResult `json:"ltc_scan,omitempty"`
	Status  string         `json:"status"`
}

type ProbeSummary struct {
	Duration          string              `json:"duration,omitempty"`
	ExistingTimecodes []TimecodeReference `json:"existing_timecodes,omitempty"`
	Video             []VideoSummary      `json:"video"`
	Audio             []AudioSummary      `json:"audio"`
	Data              []DataSummary       `json:"data"`
}

type TimecodeReference struct {
	Location string `json:"location"`
	Value    string `json:"value"`
}

type VideoSummary struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	FPS        string `json:"fps,omitempty"`
	Timecode   string `json:"timecode,omitempty"`
}

type AudioSummary struct {
	Index         int    `json:"index"`
	Codec         string `json:"codec,omitempty"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout,omitempty"`
	SampleRate    string `json:"sample_rate,omitempty"`
}

type DataSummary struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec,omitempty"`
	Handler  string `json:"handler,omitempty"`
	Timecode string `json:"timecode,omitempty"`
}

type LTCScanResult struct {
	FPS              string           `json:"fps"`
	SelectedChannel  string           `json:"selected_channel,omitempty"`
	SelectedTimecode string           `json:"selected_timecode,omitempty"`
	Channels         []LTCScanChannel `json:"channels"`
}

type LTCScanChannel struct {
	Channel  string `json:"channel"`
	Timecode string `json:"timecode,omitempty"`
	Score    int    `json:"score"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	scanLTC := fs.Bool("scan-ltc", false, "decode audio channels and report likely LTC channel")
	fps := fs.String("fps", "", "expected timecode frame rate for --scan-ltc")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"fps": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("probe requires exactly one input path")
	}
	input := fs.Arg(0)
	if err := requireInputPath(input); err != nil {
		return err
	}
	tools, err := checkRequiredTools()
	if err != nil {
		if *jsonOut {
			_ = printJSON(map[string]any{"input": input, "tools": tools, "status": "failed", "error": err.Error()})
		}
		return err
	}
	probe, err := ffprobe(context.Background(), input)
	if err != nil {
		return wrapProbeError(input, err)
	}
	result := ProbeResult{Input: input, Tools: tools, Probe: probe, Summary: summarizeProbe(probe), Status: "ok"}
	if *scanLTC {
		validFPS, err := validateFPS(*fps)
		if err != nil {
			return fmt.Errorf("--scan-ltc requires --fps: %w", err)
		}
		scan, err := scanLTCChannels(context.Background(), input, validFPS, probe)
		if err != nil {
			return err
		}
		result.LTCScan = &scan
	}
	if *jsonOut {
		return printJSON(result)
	}
	printProbe(result)
	return nil
}

func ffprobe(ctx context.Context, input string) (ProbeInfo, error) {
	stdout, _, err := runCommand(ctx, "ffprobe", "-v", "error", "-print_format", "json", "-show_format", "-show_streams", input)
	if err != nil {
		return ProbeInfo{}, err
	}
	var probe ProbeInfo
	if err := json.Unmarshal([]byte(stdout), &probe); err != nil {
		return ProbeInfo{}, fmt.Errorf("parse ffprobe JSON: %w", err)
	}
	return probe, nil
}

func printProbe(result ProbeResult) {
	fmt.Println("Input:", result.Input)
	if result.Summary.Duration != "" {
		fmt.Println("Duration:", result.Summary.Duration)
	}
	if len(result.Summary.ExistingTimecodes) > 0 {
		fmt.Println("Existing timecode:")
		for _, tc := range result.Summary.ExistingTimecodes {
			fmt.Printf("  %s: %s\n", tc.Location, tc.Value)
		}
	}
	if len(result.Summary.Video) > 0 {
		fmt.Println("Video:")
		for _, v := range result.Summary.Video {
			parts := []string{fmt.Sprintf("#%d", v.Index)}
			if v.Codec != "" {
				parts = append(parts, v.Codec)
			}
			if v.Resolution != "" {
				parts = append(parts, v.Resolution)
			}
			if v.FPS != "" {
				parts = append(parts, "fps="+v.FPS)
			}
			if v.Timecode != "" {
				parts = append(parts, "timecode="+v.Timecode)
			}
			fmt.Println(" ", strings.Join(parts, " "))
		}
	}
	if len(result.Summary.Audio) > 0 {
		fmt.Println("Audio:")
		for _, a := range result.Summary.Audio {
			parts := []string{fmt.Sprintf("#%d", a.Index)}
			if a.Codec != "" {
				parts = append(parts, a.Codec)
			}
			if a.Channels > 0 {
				parts = append(parts, fmt.Sprintf("%dch", a.Channels))
			}
			if a.ChannelLayout != "" {
				parts = append(parts, a.ChannelLayout)
			}
			if a.SampleRate != "" {
				parts = append(parts, a.SampleRate+"Hz")
			}
			fmt.Println(" ", strings.Join(parts, " "))
		}
	}
	if len(result.Summary.Data) > 0 {
		fmt.Println("Data:")
		for _, d := range result.Summary.Data {
			parts := []string{fmt.Sprintf("#%d", d.Index)}
			if d.Codec != "" {
				parts = append(parts, d.Codec)
			}
			if d.Handler != "" {
				parts = append(parts, d.Handler)
			}
			if d.Timecode != "" {
				parts = append(parts, "timecode="+d.Timecode)
			}
			fmt.Println(" ", strings.Join(parts, " "))
		}
	}
	if result.LTCScan != nil {
		fmt.Println("LTC scan:")
		for _, ch := range result.LTCScan.Channels {
			line := fmt.Sprintf("  %s: %s", ch.Channel, ch.Status)
			if ch.Timecode != "" {
				line += fmt.Sprintf(" timecode=%s score=%d", ch.Timecode, ch.Score)
			}
			if ch.Error != "" {
				line += " " + ch.Error
			}
			fmt.Println(line)
		}
		if result.LTCScan.SelectedChannel != "" {
			fmt.Printf("Recommended channel: %s (%s)\n", result.LTCScan.SelectedChannel, result.LTCScan.SelectedTimecode)
			if mismatch := timecodeMismatch(result.Summary.ExistingTimecodes, result.LTCScan.SelectedTimecode); mismatch != "" {
				fmt.Println("Warning:", mismatch)
			}
		}
	}
}

func summarizeProbe(probe ProbeInfo) ProbeSummary {
	summary := ProbeSummary{Duration: probe.Format.Duration}
	if probe.Format.Tags != nil {
		if tc := probe.Format.Tags["timecode"]; tc != "" {
			summary.ExistingTimecodes = append(summary.ExistingTimecodes, TimecodeReference{Location: "format", Value: tc})
		}
	}
	for _, s := range probe.Streams {
		tc := ""
		handler := ""
		if s.Tags != nil {
			tc = s.Tags["timecode"]
			handler = s.Tags["handler_name"]
			if tc != "" {
				summary.ExistingTimecodes = append(summary.ExistingTimecodes, TimecodeReference{Location: fmt.Sprintf("stream #%d", s.Index), Value: tc})
			}
		}
		switch s.CodecType {
		case "video":
			summary.Video = append(summary.Video, VideoSummary{
				Index: s.Index, Codec: s.CodecName, Resolution: resolution(s), FPS: streamFPS(s), Timecode: tc,
			})
		case "audio":
			summary.Audio = append(summary.Audio, AudioSummary{
				Index: s.Index, Codec: s.CodecName, Channels: s.Channels, ChannelLayout: s.ChannelLayout, SampleRate: s.SampleRate,
			})
		case "data":
			summary.Data = append(summary.Data, DataSummary{Index: s.Index, Codec: s.CodecName, Handler: handler, Timecode: tc})
		}
	}
	return summary
}

func scanLTCChannels(ctx context.Context, input, fps string, probe ProbeInfo) (LTCScanResult, error) {
	tempDir, err := os.MkdirTemp("", "tcforge-probe-*")
	if err != nil {
		return LTCScanResult{}, err
	}
	defer os.RemoveAll(tempDir)

	scan := LTCScanResult{FPS: fps}
	candidates := ltcChannelCandidates(probe)
	for _, candidate := range candidates {
		wavPath := filepath.Join(tempDir, candidate.file)
		ch := LTCScanChannel{Channel: candidate.channel, Status: "not found"}
		extract := buildExtractCommand(input, wavPath, candidate.audioMap, candidate.panChannel)
		if _, _, err := runCommand(ctx, extract.Program, extract.Args...); err != nil {
			ch.Status = "error"
			ch.Error = err.Error()
			scan.Channels = append(scan.Channels, ch)
			continue
		}
		ltcDump := buildLTCDumpCommand(wavPath, fps)
		stdout, stderr, err := runCommand(ctx, ltcDump.Program, ltcDump.Args...)
		if err != nil {
			ch.Status = "error"
			ch.Error = err.Error()
			scan.Channels = append(scan.Channels, ch)
			continue
		}
		output := stdout + "\n" + stderr
		ch.Score = ltcDecodeScore(output)
		tc, err := parseLTCStart(output)
		if err != nil || ch.Score == 0 {
			scan.Channels = append(scan.Channels, ch)
			continue
		}
		ch.Status = "found"
		ch.Timecode = tc
		scan.Channels = append(scan.Channels, ch)
		if scan.SelectedChannel == "" || ch.Score > selectedScore(scan) {
			scan.SelectedChannel = ch.Channel
			scan.SelectedTimecode = ch.Timecode
		}
	}
	return scan, nil
}

func selectedScore(scan LTCScanResult) int {
	for _, ch := range scan.Channels {
		if ch.Channel == scan.SelectedChannel {
			return ch.Score
		}
	}
	return 0
}

func resolution(s StreamInfo) string {
	if s.Width <= 0 || s.Height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", s.Width, s.Height)
}

func streamFPS(s StreamInfo) string {
	if s.AvgFrameRate != "" && s.AvgFrameRate != "0/0" {
		return s.AvgFrameRate
	}
	return s.RFrameRate
}

func timecodeMismatch(existing []TimecodeReference, decoded string) string {
	if decoded == "" {
		return ""
	}
	normalizedDecoded := normalizeTimecodeSeparators(decoded)
	for _, tc := range existing {
		if normalizeTimecodeSeparators(tc.Value) != normalizedDecoded {
			return fmt.Sprintf("decoded LTC %s differs from existing %s timecode %s", decoded, tc.Location, tc.Value)
		}
	}
	return ""
}

func normalizeTimecodeSeparators(tc string) string {
	return strings.ReplaceAll(tc, ";", ":")
}

func firstTimecode(probe ProbeInfo) string {
	if probe.Format.Tags != nil {
		if tc := probe.Format.Tags["timecode"]; tc != "" {
			return tc
		}
	}
	for _, s := range probe.Streams {
		if s.Tags != nil {
			if tc := s.Tags["timecode"]; tc != "" {
				return tc
			}
		}
	}
	return ""
}
