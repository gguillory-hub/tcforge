package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	Input  string       `json:"input"`
	Tools  []ToolStatus `json:"tools"`
	Probe  ProbeInfo    `json:"probe"`
	Status string       `json:"status"`
}

func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
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
		return err
	}
	result := ProbeResult{Input: input, Tools: tools, Probe: probe, Status: "ok"}
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
	if result.Probe.Format.Duration != "" {
		fmt.Println("Duration:", result.Probe.Format.Duration)
	}
	if tc := firstTimecode(result.Probe); tc != "" {
		fmt.Println("Existing timecode:", tc)
	}
	fmt.Println("Streams:")
	for _, s := range result.Probe.Streams {
		parts := []string{fmt.Sprintf("#%d", s.Index), s.CodecType}
		if s.CodecName != "" {
			parts = append(parts, s.CodecName)
		}
		if s.CodecType == "video" {
			if s.Width > 0 && s.Height > 0 {
				parts = append(parts, fmt.Sprintf("%dx%d", s.Width, s.Height))
			}
			if s.AvgFrameRate != "" {
				parts = append(parts, "fps="+s.AvgFrameRate)
			}
		}
		if s.CodecType == "audio" {
			if s.Channels > 0 {
				parts = append(parts, fmt.Sprintf("%dch", s.Channels))
			}
			if s.ChannelLayout != "" {
				parts = append(parts, s.ChannelLayout)
			}
			if s.SampleRate != "" {
				parts = append(parts, s.SampleRate+"Hz")
			}
		}
		fmt.Println(" ", strings.Join(parts, " "))
	}
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
