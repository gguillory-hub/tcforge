package tcforge

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BatchResult struct {
	InputDir  string        `json:"input_dir"`
	OutputDir string        `json:"output_dir"`
	Results   []WriteResult `json:"results"`
	Status    string        `json:"status"`
}

func runBatch(args []string) error {
	fs := flag.NewFlagSet("batch", flag.ContinueOnError)
	channel := fs.String("channel", "auto", "audio channel containing LTC: auto, left, right, 1, or 2")
	fps := fs.String("fps", "", "timecode frame rate")
	outputDir := fs.String("output-dir", "", "output folder")
	clean := fs.Bool("clean", false, "write clean NLE timecode files with video plus generated tmcd only")
	drop := fs.Bool("drop-ltc-audio", false, "drop the first audio stream instead of preserving it")
	overwrite := fs.Bool("overwrite", false, "overwrite outputs if they exist")
	dryRun := fs.Bool("dry-run", false, "print planned commands without writing media")
	jsonOut := fs.Bool("json", false, "emit JSON")
	verbose := fs.Bool("verbose", false, "print command details")
	if err := fs.Parse(flagsFirst(args, map[string]bool{
		"channel":    true,
		"fps":        true,
		"output-dir": true,
	})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("batch requires exactly one input folder")
	}
	inputDir := fs.Arg(0)
	if *outputDir == "" {
		return fmt.Errorf("--output-dir is required")
	}
	files, err := mediaFiles(inputDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no supported media files found in %s", inputDir)
	}
	if !*dryRun {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			return err
		}
	}
	batch := BatchResult{InputDir: inputDir, OutputDir: *outputDir, Status: "ok"}
	var failed int
	for _, file := range files {
		base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)) + ".mov"
		out := filepath.Join(*outputDir, base)
		result, err := writeOne(context.Background(), WriteOptions{
			Input: file, Output: out, Channel: *channel, FPS: *fps,
			Clean: *clean, DropLTCAudio: *drop, Overwrite: *overwrite, DryRun: *dryRun,
		})
		if err != nil {
			failed++
		}
		batch.Results = append(batch.Results, result)
	}
	if failed > 0 {
		batch.Status = "partial-failure"
	}
	if *jsonOut {
		_ = printJSON(batch)
	} else {
		printBatchResult(batch, *verbose)
	}
	if failed > 0 {
		return fmt.Errorf("%d file(s) failed", failed)
	}
	return nil
}

func mediaFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if isSupportedMedia(path) {
			files = append(files, path)
		}
	}
	return files, nil
}

func isSupportedMedia(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".m4v", ".mxf":
		return true
	default:
		return false
	}
}

func printBatchResult(batch BatchResult, verbose bool) {
	fmt.Println("Input dir:", batch.InputDir)
	fmt.Println("Output dir:", batch.OutputDir)
	fmt.Println("Status:", batch.Status)
	for _, result := range batch.Results {
		line := fmt.Sprintf("%s -> %s [%s]", result.Input, result.Output, result.Status)
		if result.Error != "" {
			line += " " + result.Error
		}
		fmt.Println(line)
		if verbose {
			for _, cmd := range result.Commands {
				fmt.Printf("  %s %v\n", cmd.Program, cmd.Args)
			}
		}
	}
}
