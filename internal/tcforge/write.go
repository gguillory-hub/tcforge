package tcforge

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type WriteOptions struct {
	Input            string                                          `json:"input"`
	Output           string                                          `json:"output"`
	Channel          string                                          `json:"channel"`
	FPS              string                                          `json:"fps"`
	Clean            bool                                            `json:"clean"`
	DropLTCAudio     bool                                            `json:"drop_ltc_audio"`
	Overwrite        bool                                            `json:"overwrite"`
	DryRun           bool                                            `json:"dry_run"`
	AllowFPSMismatch bool                                            `json:"allow_fps_mismatch"`
	JSON             bool                                            `json:"-"`
	Verbose          bool                                            `json:"-"`
	Progress         func(stage string, percent float64, exact bool) `json:"-"`
}

type WriteResult struct {
	Input            string           `json:"input"`
	Output           string           `json:"output"`
	DetectedStreams  []StreamInfo     `json:"detected_streams"`
	RequestedChannel string           `json:"requested_ltc_channel"`
	SelectedChannel  string           `json:"selected_ltc_channel"`
	DecodedStartTC   string           `json:"decoded_start_timecode,omitempty"`
	FPS              string           `json:"fps"`
	Commands         []CommandSummary `json:"commands"`
	Status           string           `json:"status"`
	ErrorCode        string           `json:"error_code,omitempty"`
	Error            string           `json:"error,omitempty"`
	Suggestion       string           `json:"suggestion,omitempty"`
}

func runWrite(args []string) error {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	channel := fs.String("channel", "auto", "audio channel containing LTC: auto, left, right, or a channel number")
	fps := fs.String("fps", "", "timecode frame rate")
	output := fs.String("output", "", "output media path")
	clean := fs.Bool("clean", false, "write a clean NLE timecode file with video plus generated tmcd only")
	drop := fs.Bool("drop-ltc-audio", false, "drop the first audio stream instead of preserving it")
	overwrite := fs.Bool("overwrite", false, "overwrite output if it exists")
	dryRun := fs.Bool("dry-run", false, "print planned commands without writing media")
	allowFPSMismatch := fs.Bool("allow-fps-mismatch", false, "allow writing when requested timecode fps differs from video fps")
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
		Clean: *clean, DropLTCAudio: *drop, Overwrite: *overwrite, DryRun: *dryRun, AllowFPSMismatch: *allowFPSMismatch, JSON: *jsonOut, Verbose: *verbose,
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
		Input:            options.Input,
		Output:           options.Output,
		RequestedChannel: options.Channel,
		SelectedChannel:  options.Channel,
		FPS:              options.FPS,
		Status:           "failed",
	}
	fail := func(err error) (WriteResult, error) {
		code, suggestion := appErrorFields(err)
		result.ErrorCode = code
		result.Error = appErrorSummary(err)
		result.Suggestion = suggestion
		return result, err
	}

	if err := requireInputPath(options.Input); err != nil {
		return fail(err)
	}
	if options.Output == "" {
		return fail(appError("missing_output", "--output is required.", "Pass an output path, or use the beginner-friendly fix command which creates one automatically.", nil))
	}
	if samePath(options.Input, options.Output) {
		return fail(appError("output_matches_input", "Output must not be the same as input.", "Choose a new output path so the original camera file is never modified.", nil))
	}
	if _, err := os.Stat(options.Output); err == nil && !options.Overwrite {
		return fail(appError("output_exists", fmt.Sprintf("Output already exists: %s", options.Output), "Choose a different output path or pass --overwrite.", nil))
	}
	if err := ensureOutputWritable(options.Output, options.Overwrite); err != nil {
		return fail(err)
	}
	channel, _, err := normalizeChannel(options.Channel)
	if err != nil {
		return fail(err)
	}
	result.RequestedChannel = channel
	result.SelectedChannel = channel
	fps, err := validateFPS(options.FPS)
	if err != nil {
		return fail(err)
	}
	result.FPS = fps
	if _, err := checkRequiredTools(); err != nil {
		return fail(err)
	}
	emitProgress(options, "Reading media metadata", 0.05, false)
	probe, err := ffprobe(ctx, options.Input)
	if err != nil {
		return fail(wrapProbeError(options.Input, err))
	}
	result.DetectedStreams = probe.Streams
	if err := ensureVideoStream(probe); err != nil {
		return fail(err)
	}
	if !options.AllowFPSMismatch {
		if err := ensureFPSMatch(probe, fps); err != nil {
			return fail(err)
		}
	}
	if err := ensureAudioChannel(probe, channel); err != nil {
		return fail(err)
	}

	tempDir, err := os.MkdirTemp("", "tcforge-*")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(tempDir)

	if options.DryRun {
		result.DecodedStartTC = "dry-run"
		if channel == "auto" {
			result.SelectedChannel = "auto"
			for _, candidate := range ltcChannelCandidates(probe) {
				wavPath := filepath.Join(tempDir, candidate.file)
				result.Commands = append(result.Commands,
					buildExtractCommand(options.Input, wavPath, candidate.audioMap, candidate.panChannel),
					buildLTCDumpCommand(wavPath, fps),
				)
			}
		} else {
			candidate, _ := findLTCChannelCandidate(probe, channel)
			wavPath := filepath.Join(tempDir, "ltc.wav")
			result.Commands = append(result.Commands,
				buildExtractCommand(options.Input, wavPath, candidate.audioMap, candidate.panChannel),
				buildLTCDumpCommand(wavPath, fps),
			)
		}
		writeCmd := buildWriteCommand(options, "00:00:00:00")
		result.Commands = append(result.Commands, writeCmd)
		result.Status = "dry-run"
		return result, nil
	}

	emitProgress(options, "Decoding audio LTC", 0.2, false)
	decode, err := decodeLTC(ctx, options.Input, tempDir, channel, fps, probe)
	if err != nil {
		return fail(err)
	}
	result.Commands = append(result.Commands, decode.Commands...)
	result.SelectedChannel = decode.Channel
	result.DecodedStartTC = decode.Timecode
	writeCmd := buildWriteCommand(options, decode.Timecode)
	result.Commands = append(result.Commands, writeCmd)
	emitProgress(options, "Writing output file", 0, true)
	if _, _, err := runCommandWithProgress(ctx, writeCmd.Program, writeCmd.Args, probe.Format.Duration, func(percent float64) {
		emitProgress(options, "Writing output file", percent, true)
	}); err != nil {
		return fail(err)
	}
	emitProgress(options, "Done", 1, true)
	result.Status = "ok"
	return result, nil
}

func emitProgress(options WriteOptions, stage string, percent float64, exact bool) {
	if options.Progress != nil {
		options.Progress(stage, percent, exact)
	}
}

type decodeResult struct {
	Channel  string
	Timecode string
	Score    int
	Commands []CommandSummary
}

func decodeLTC(ctx context.Context, input, tempDir, channel, fps string, probe ProbeInfo) (decodeResult, error) {
	if channel != "auto" {
		candidate, ok := findLTCChannelCandidate(probe, channel)
		if !ok {
			return decodeResult{}, appError("audio_channel_missing", fmt.Sprintf("%s requested, but input has %d audio channel(s).", displayChannel(channel), audioChannelCount(probe)), "Use --channel auto, or run probe --scan-ltc --fps <fps> to let tcforge inspect all available audio streams.", nil)
		}
		wavPath := filepath.Join(tempDir, "ltc.wav")
		extract := buildExtractCommand(input, wavPath, candidate.audioMap, candidate.panChannel)
		ltcDump := buildLTCDumpCommand(wavPath, fps)
		if _, _, err := runCommand(ctx, extract.Program, extract.Args...); err != nil {
			return decodeResult{}, err
		}
		stdout, stderr, err := runCommand(ctx, ltcDump.Program, ltcDump.Args...)
		if err != nil {
			return decodeResult{}, err
		}
		output := stdout + "\n" + stderr
		tc, err := parseLTCStart(output)
		if err != nil {
			return decodeResult{}, appError(
				"ltc_not_found",
				fmt.Sprintf("No valid LTC found on the %s channel.", channel),
				"Try --channel auto, check the timecode audio connection, or run probe --scan-ltc --fps <fps> for details.",
				err,
			)
		}
		return decodeResult{
			Channel: channel, Timecode: tc, Score: ltcDecodeScore(output),
			Commands: []CommandSummary{extract, ltcDump},
		}, nil
	}

	candidates := ltcChannelCandidates(probe)
	results := make([]decodeResult, 0, len(candidates))
	var commands []CommandSummary
	for _, candidate := range candidates {
		wavPath := filepath.Join(tempDir, candidate.file)
		extract := buildExtractCommand(input, wavPath, candidate.audioMap, candidate.panChannel)
		ltcDump := buildLTCDumpCommand(wavPath, fps)
		commands = append(commands, extract, ltcDump)
		if _, _, err := runCommand(ctx, extract.Program, extract.Args...); err != nil {
			continue
		}
		stdout, stderr, err := runCommand(ctx, ltcDump.Program, ltcDump.Args...)
		if err != nil {
			continue
		}
		output := stdout + "\n" + stderr
		tc, err := parseLTCStart(output)
		if err != nil {
			continue
		}
		score := ltcDecodeScore(output)
		if score == 0 {
			continue
		}
		results = append(results, decodeResult{Channel: candidate.channel, Timecode: tc, Score: score})
	}
	if len(results) == 0 {
		return decodeResult{Commands: commands}, appError(
			"ltc_not_found",
			fmt.Sprintf("Auto channel detection failed: no valid LTC found on %d audio channel(s).", audioChannelCount(probe)),
			"Check that the timecode box was connected to the camera audio input, the audio level was not muted/clipped, and the correct file was selected. You can run probe --scan-ltc --fps <fps> for details.",
			nil,
		)
	}
	best := results[0]
	if len(results) > 1 && results[1].Score > best.Score {
		best = results[1]
	}
	best.Commands = commands
	return best, nil
}

func buildLTCDumpCommand(wavPath, fps string) CommandSummary {
	return CommandSummary{Program: "ltcdump", Args: []string{"--fps", ltcDumpFPS(fps), wavPath}}
}

func buildExtractCommand(input, wavPath, audioMap, panChannel string) CommandSummary {
	if audioMap == "" {
		audioMap = "0:a:0"
	}
	return CommandSummary{
		Program: "ffmpeg",
		Args: []string{
			"-y",
			"-i", input,
			"-vn",
			"-map", audioMap,
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
	)
	if options.Clean {
		args = append(args, "-map", "0:v:0")
	} else {
		args = append(args, "-map", "0")
	}
	if options.DropLTCAudio && !options.Clean {
		args = append(args, "-map", "-0:a:0")
	}
	args = append(args,
		"-c", "copy",
		"-timecode", timecode,
		"-metadata", "timecode="+timecode,
		"-metadata", "tcforge=1",
		"-write_tmcd", "on",
		options.Output,
	)
	return CommandSummary{Program: "ffmpeg", Args: args}
}

func ensureAudioChannel(probe ProbeInfo, channel string) error {
	if audioChannelCount(probe) == 0 {
		return appError("audio_missing", "No audio stream found.", "Audio LTC must be recorded on a camera audio track. Choose a file with audio LTC, or use a camera file that already has metadata timecode.", nil)
	}
	if channel == "auto" {
		return nil
	}
	if _, ok := findLTCChannelCandidate(probe, channel); !ok {
		return appError("audio_channel_missing", fmt.Sprintf("%s requested, but input has %d audio channel(s).", displayChannel(channel), audioChannelCount(probe)), "Use --channel auto, or run probe --scan-ltc --fps <fps> to let tcforge inspect all available audio streams.", nil)
	}
	return nil
}

func ensureVideoStream(probe ProbeInfo) error {
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			return nil
		}
	}
	return appError("video_missing", "No video stream found.", "Choose a valid video file. Audio-only files cannot receive video timecode metadata.", nil)
}

func ensureFPSMatch(probe ProbeInfo, requestedFPS string) error {
	for _, s := range probe.Streams {
		if s.CodecType != "video" {
			continue
		}
		videoFPS := streamFPS(s)
		if videoFPS == "" || videoFPS == "0/0" {
			return nil
		}
		if !fpsMatches(videoFPS, requestedFPS) {
			return appError(
				"fps_mismatch",
				fmt.Sprintf("Requested timecode fps %s does not match video fps %s.", requestedFPS, videoFPS),
				"Use the video's actual frame rate for --fps, or pass --allow-fps-mismatch if you are intentionally handling an edge case.",
				nil,
			)
		}
		return nil
	}
	return nil
}

func ensureOutputWritable(output string, overwrite bool) error {
	dir := filepath.Dir(output)
	if dir == "" || dir == "." {
		dir = "."
	}
	info, err := os.Stat(dir)
	if err != nil {
		return appError("output_directory_missing", fmt.Sprintf("Output directory does not exist: %s", dir), "Create the output folder or choose a different --output path.", err)
	}
	if !info.IsDir() {
		return appError("output_directory_invalid", fmt.Sprintf("Output parent is not a directory: %s", dir), "Choose an output path inside a real folder.", nil)
	}
	temp, err := os.CreateTemp(dir, ".tcforge-write-test-*")
	if err != nil {
		return appError("output_not_writable", fmt.Sprintf("Cannot write to output directory: %s", dir), "Check folder permissions, close apps that may be locking the folder, or choose a different output location.", err)
	}
	name := temp.Name()
	_ = temp.Close()
	_ = os.Remove(name)
	return nil
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
	if result.Status == "ok" || result.Status == "dry-run" {
		fmt.Println("Mode:", writeMode(result.Commands))
	}
	fmt.Println("Timecode:", result.DecodedStartTC)
	fmt.Println("Status:", result.Status)
	if verbose {
		fmt.Println("Commands:")
		for _, cmd := range result.Commands {
			fmt.Printf("  %s %v\n", cmd.Program, cmd.Args)
		}
	}
}

func writeMode(commands []CommandSummary) string {
	for _, cmd := range commands {
		if cmd.Program != "ffmpeg" {
			continue
		}
		for i, arg := range cmd.Args {
			if arg == "-map" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "0:v:0" {
				return "clean"
			}
		}
	}
	return "preserve"
}
