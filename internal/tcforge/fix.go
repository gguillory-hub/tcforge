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

type FixResult struct {
	Inputs    []string      `json:"inputs"`
	OutputDir string        `json:"output_dir,omitempty"`
	Results   []WriteResult `json:"results"`
	Succeeded int           `json:"succeeded"`
	Failed    int           `json:"failed"`
	Status    string        `json:"status"`
}

type FixJob struct {
	Input            string `json:"input"`
	Output           string `json:"output,omitempty"`
	FPS              string `json:"fps,omitempty"`
	Channel          string `json:"channel,omitempty"`
	Clean            *bool  `json:"clean,omitempty"`
	Preserve         *bool  `json:"preserve,omitempty"`
	Overwrite        *bool  `json:"overwrite,omitempty"`
	AllowFPSMismatch *bool  `json:"allow_fps_mismatch,omitempty"`
}

func runFix(args []string) error {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fps := fs.String("fps", "", "timecode frame rate")
	output := fs.String("output", "", "output media path")
	outputDir := fs.String("output-dir", "", "output folder for multi-file runs")
	manifest := fs.String("manifest", "", "JSON manifest containing per-file fix jobs")
	channel := fs.String("channel", "auto", "audio channel containing LTC: auto, left, right, or a channel number")
	preserve := fs.Bool("preserve", false, "preserve input streams instead of writing clean video plus tmcd")
	overwrite := fs.Bool("overwrite", false, "overwrite output if it exists")
	dryRun := fs.Bool("dry-run", false, "print planned commands without writing media")
	allowFPSMismatch := fs.Bool("allow-fps-mismatch", false, "allow writing when requested timecode fps differs from video fps")
	jsonOut := fs.Bool("json", false, "emit JSON")
	verbose := fs.Bool("verbose", false, "print command details")
	if err := fs.Parse(flagsFirst(args, map[string]bool{
		"fps":        true,
		"output":     true,
		"output-dir": true,
		"manifest":   true,
		"channel":    true,
	})); err != nil {
		return err
	}
	if *manifest != "" && fs.NArg() > 0 {
		return appError("manifest_with_inputs", "--manifest cannot be combined with positional input files.", "Use either --manifest jobs.json or pass files directly, not both.", nil)
	}
	var jobs []FixJob
	if *manifest != "" {
		loadedJobs, err := loadFixManifest(*manifest)
		if err != nil {
			return err
		}
		jobs = loadedJobs
	} else if fs.NArg() > 0 {
		for _, input := range fs.Args() {
			jobs = append(jobs, FixJob{Input: input})
		}
	}
	if len(jobs) == 0 {
		return appError("missing_input", "fix requires at least one input path.", "Pass one or more video files, for example: tcforge fix C1315.MP4 C1316.MP4", nil)
	}
	if len(jobs) > 1 && *output != "" {
		return appError("output_invalid_for_multiple_inputs", "--output can only be used with one input file.", "For multiple files, use --output-dir or let tcforge write beside each input file.", nil)
	}
	if *outputDir != "" && !*dryRun {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			return appError("output_directory_not_writable", fmt.Sprintf("Could not create output directory: %s", *outputDir), "Check folder permissions or choose a different --output-dir.", err)
		}
	}

	fix := FixResult{OutputDir: *outputDir, Status: "ok"}
	seenOutputs := map[string]string{}
	for _, job := range jobs {
		input := job.Input
		if input == "" {
			err := appError("manifest_input_missing", "Manifest job is missing an input path.", "Each manifest job must include an input field.", nil)
			fix.Results = append(fix.Results, failedWriteResult("", "", coalesceString(job.Channel, *channel), err))
			fix.Failed++
			continue
		}
		fix.Inputs = append(fix.Inputs, input)
		out := outputForFixJob(job, *output, *outputDir)
		if previous, exists := seenOutputs[filepath.Clean(out)]; exists {
			err := appError("duplicate_output", fmt.Sprintf("Multiple inputs would write the same output: %s", out), fmt.Sprintf("Inputs %s and %s collide. Use distinct input names or a manifest with explicit outputs.", previous, input), nil)
			fix.Results = append(fix.Results, failedWriteResult(input, out, effectiveChannel(job, *channel), err))
			fix.Failed++
			continue
		}
		seenOutputs[filepath.Clean(out)] = input
		selectedFPS := coalesceString(job.FPS, *fps)
		if selectedFPS == "" {
			inferredFPS, err := inferFixFPS(context.Background(), input)
			if err != nil {
				fix.Results = append(fix.Results, failedWriteResult(input, out, effectiveChannel(job, *channel), err))
				fix.Failed++
				continue
			}
			selectedFPS = inferredFPS
		}
		options := WriteOptions{
			Input: input, Output: out, Channel: effectiveChannel(job, *channel), FPS: selectedFPS,
			Clean: effectiveClean(job, *preserve), Overwrite: effectiveBool(job.Overwrite, *overwrite), DryRun: *dryRun,
			AllowFPSMismatch: effectiveBool(job.AllowFPSMismatch, *allowFPSMismatch), JSON: *jsonOut, Verbose: *verbose,
		}
		result, err := writeOne(context.Background(), options)
		if err != nil {
			fix.Failed++
		} else {
			fix.Succeeded++
		}
		fix.Results = append(fix.Results, result)
	}
	if fix.Failed > 0 {
		fix.Status = "partial-failure"
	}
	if fix.Succeeded == 0 && fix.Failed > 0 {
		fix.Status = "failed"
	}
	if *jsonOut {
		_ = printJSON(fix)
	} else {
		printFixResult(fix, *verbose)
	}
	if fix.Failed > 0 {
		return appError("fix_partial_failure", fmt.Sprintf("%d file(s) failed, %d succeeded.", fix.Failed, fix.Succeeded), "Review the failed rows above. Run probe --scan-ltc on failed files for more detail.", nil)
	}
	return nil
}

func outputForFixInput(input, output, outputDir string) string {
	if output != "" {
		return output
	}
	if outputDir != "" {
		return filepath.Join(outputDir, filepath.Base(defaultFixedOutput(input)))
	}
	return defaultFixedOutput(input)
}

func outputForFixJob(job FixJob, output, outputDir string) string {
	if job.Output != "" {
		return job.Output
	}
	return outputForFixInput(job.Input, output, outputDir)
}

func defaultFixedOutput(input string) string {
	ext := filepath.Ext(input)
	base := strings.TrimSuffix(input, ext)
	return base + "_tcforge.mov"
}

func failedWriteResult(input, output, channel string, err error) WriteResult {
	code, suggestion := appErrorFields(err)
	return WriteResult{
		Input: input, Output: output, RequestedChannel: channel, SelectedChannel: channel,
		Status: "failed", ErrorCode: code, Error: appErrorSummary(err), Suggestion: suggestion,
	}
}

func loadFixManifest(path string) ([]FixJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, appError("manifest_unreadable", fmt.Sprintf("Could not read manifest: %s", path), "Check the manifest path and file permissions.", err)
	}
	var jobs []FixJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, appError("manifest_invalid", fmt.Sprintf("Could not parse JSON manifest: %s", path), "Use a JSON array of jobs, each with at least an input field.", err)
	}
	if len(jobs) == 0 {
		return nil, appError("manifest_empty", "Manifest contains no jobs.", "Add at least one job with an input field.", nil)
	}
	return jobs, nil
}

func effectiveChannel(job FixJob, fallback string) string {
	return coalesceString(job.Channel, fallback)
}

func effectiveClean(job FixJob, preserveFlag bool) bool {
	if job.Clean != nil {
		return *job.Clean
	}
	if job.Preserve != nil {
		return !*job.Preserve
	}
	return !preserveFlag
}

func effectiveBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func coalesceString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func printFixResult(fix FixResult, verbose bool) {
	for _, result := range fix.Results {
		prefix := "OK"
		if result.Status != "ok" && result.Status != "dry-run" {
			prefix = "FAILED"
		}
		line := fmt.Sprintf("%-6s %s -> %s", prefix, result.Input, result.Output)
		if result.DecodedStartTC != "" && result.DecodedStartTC != "dry-run" {
			line += fmt.Sprintf(" timecode=%s channel=%s", result.DecodedStartTC, result.SelectedChannel)
		}
		if result.Error != "" {
			line += " " + result.Error
		}
		fmt.Println(line)
		if result.Suggestion != "" {
			fmt.Println("       Suggestion:", result.Suggestion)
		}
		if verbose {
			for _, cmd := range result.Commands {
				fmt.Printf("       %s %v\n", cmd.Program, cmd.Args)
			}
		}
	}
	fmt.Printf("%d succeeded, %d failed\n", fix.Succeeded, fix.Failed)
}

func inferFixFPS(ctx context.Context, input string) (string, error) {
	if err := requireInputPath(input); err != nil {
		return "", err
	}
	if _, err := checkRequiredTools(); err != nil {
		return "", err
	}
	probe, err := ffprobe(ctx, input)
	if err != nil {
		return "", wrapProbeError(input, err)
	}
	for _, stream := range probe.Streams {
		if stream.CodecType != "video" {
			continue
		}
		fps := canonicalFPS(streamFPS(stream))
		if fps == "" {
			return "", appError(
				"fps_not_detected",
				"Could not infer video fps from the input file.",
				"Pass --fps manually, for example: --fps 29.97",
				nil,
			)
		}
		return fps, nil
	}
	return "", appError("video_missing", "No video stream found.", "Choose a valid video file. Audio-only files cannot receive video timecode metadata.", nil)
}
