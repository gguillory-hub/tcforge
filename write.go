package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type WriteOptions struct {
	Input        string `json:"input"`
	Output       string `json:"output"`
	Channel      string `json:"channel"`
	FPS          string `json:"fps"`
	DropLTCAudio bool   `json:"drop_ltc_audio"`
	Overwrite    bool   `json:"overwrite"`
	DryRun       bool   `json:"dry_run"`
	JSON         bool   `json:"-"`
	Verbose      bool   `json:"-"`
}

type WriteResult struct {
	Input           string           `json:"input"`
	Output          string           `json:"output"`
	DetectedStreams []StreamInfo     `json:"detected_streams"`
	SelectedChannel string           `json:"selected_ltc_channel"`
	DecodedStartTC  string           `json:"decoded_start_timecode,omitempty"`
	FPS             string           `json:"fps"`
	Commands        []CommandSummary `json:"commands"`
	Status          string           `json:"status"`
	Error           string           `json:"error,omitempty"`
}

func runWrite(args []string) error {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	channel := fs.String("channel", "", "audio channel containing LTC: left, right, 1, or 2")
	fps := fs.String("fps", "", "timecode frame rate")
	output := fs.String("output", "", "output media path")
	drop := fs.Bool("drop-ltc-audio", false, "drop the first audio stream instead of preserving it")
	overwrite := fs.Bool("overwrite", false, "overwrite output if it exists")
	dryRun := fs.Bool("dry-run", false, "print planned commands without writing media")
	jsonOut := fs.Bool("json", false, "emit JSON")
	verbose := fs.Bool("verbose", false, "print command details")
	if err := fs.Parse(flagsFirst(args, map[string]bool{
		"channel": true,
		"fps":     true,
		"output":  true,
	})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("write requires exactly one input path")
	}
	options := WriteOptions{
		Input: fs.Arg(0), Output: *output, Channel: *channel, FPS: *fps,
		DropLTCAudio: *drop, Overwrite: *overwrite, DryRun: *dryRun, JSON: *jsonOut, Verbose: *verbose,
	}
	result, err := writeOne(context.Background(), options)
	if *jsonOut {
		_ = printJSON(result)
	}
	if err != nil {
		return err
	}
	if !*jsonOut {
		printWriteResult(result, *verbose)
	}
	return nil
}

func writeOne(ctx context.Context, options WriteOptions) (WriteResult, error) {
	result := WriteResult{
		Input:           options.Input,
		Output:          options.Output,
		SelectedChannel: options.Channel,
		FPS:             options.FPS,
		Status:          "failed",
	}
	fail := func(err error) (WriteResult, error) {
		result.Error = err.Error()
		return result, err
	}

	if err := requireInputPath(options.Input); err != nil {
		return fail(err)
	}
	if options.Output == "" {
		return fail(fmt.Errorf("--output is required"))
	}
	if samePath(options.Input, options.Output) {
		return fail(fmt.Errorf("output must not be the same as input"))
	}
	if _, err := os.Stat(options.Output); err == nil && !options.Overwrite {
		return fail(fmt.Errorf("output exists; pass --overwrite to replace it"))
	}
	channel, panChannel, err := normalizeChannel(options.Channel)
	if err != nil {
		return fail(err)
	}
	result.SelectedChannel = channel
	fps, err := validateFPS(options.FPS)
	if err != nil {
		return fail(err)
	}
	result.FPS = fps
	if _, err := checkRequiredTools(); err != nil {
		return fail(err)
	}
	probe, err := ffprobe(ctx, options.Input)
	if err != nil {
		return fail(err)
	}
	result.DetectedStreams = probe.Streams
	if err := ensureAudioChannel(probe, channel); err != nil {
		return fail(err)
	}

	tempDir, err := os.MkdirTemp("", "ltc2meta-*")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(tempDir)
	wavPath := filepath.Join(tempDir, "ltc.wav")

	extract := buildExtractCommand(options.Input, wavPath, panChannel)
	result.Commands = append(result.Commands, extract)

	if options.DryRun {
		result.DecodedStartTC = "dry-run"
		writeCmd := buildWriteCommand(options, "00:00:00:00")
		result.Commands = append(result.Commands, writeCmd)
		result.Status = "dry-run"
		return result, nil
	}

	if _, _, err := runCommand(ctx, extract.Program, extract.Args...); err != nil {
		return fail(err)
	}
	stdout, stderr, err := runCommand(ctx, "ltcdump", wavPath)
	ltcOutput := stdout + "\n" + stderr
	if err != nil {
		return fail(err)
	}
	tc, err := parseLTCStart(ltcOutput)
	if err != nil {
		return fail(err)
	}
	result.DecodedStartTC = tc
	writeCmd := buildWriteCommand(options, tc)
	result.Commands = append(result.Commands, writeCmd)
	if _, _, err := runCommand(ctx, writeCmd.Program, writeCmd.Args...); err != nil {
		return fail(err)
	}
	result.Status = "ok"
	return result, nil
}

func buildExtractCommand(input, wavPath, panChannel string) CommandSummary {
	return CommandSummary{
		Program: "ffmpeg",
		Args: []string{
			"-y",
			"-i", input,
			"-vn",
			"-map", "0:a:0",
			"-af", "pan=mono|c0=" + panChannel,
			"-ac", "1",
			"-ar", "48000",
			wavPath,
		},
	}
}

func buildWriteCommand(options WriteOptions, timecode string) CommandSummary {
	args := []string{"-y"}
	if !options.Overwrite {
		args = []string{"-n"}
	}
	args = append(args,
		"-i", options.Input,
		"-map", "0",
	)
	if options.DropLTCAudio {
		args = append(args, "-map", "-0:a:0")
	}
	args = append(args,
		"-c", "copy",
		"-timecode", timecode,
		"-metadata", "timecode="+timecode,
		"-write_tmcd", "on",
		options.Output,
	)
	return CommandSummary{Program: "ffmpeg", Args: args}
}

func ensureAudioChannel(probe ProbeInfo, channel string) error {
	for _, s := range probe.Streams {
		if s.CodecType == "audio" {
			if channel == "right" && s.Channels < 2 {
				return fmt.Errorf("right channel requested, but first audio stream has %d channel(s)", s.Channels)
			}
			return nil
		}
	}
	return fmt.Errorf("no audio stream found")
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return filepath.Clean(aa) == filepath.Clean(bb)
}

func printWriteResult(result WriteResult, verbose bool) {
	fmt.Println("Input:", result.Input)
	fmt.Println("Output:", result.Output)
	fmt.Println("Channel:", result.SelectedChannel)
	fmt.Println("FPS:", result.FPS)
	fmt.Println("Timecode:", result.DecodedStartTC)
	fmt.Println("Status:", result.Status)
	if verbose {
		fmt.Println("Commands:")
		for _, cmd := range result.Commands {
			fmt.Printf("  %s %v\n", cmd.Program, cmd.Args)
		}
	}
}
