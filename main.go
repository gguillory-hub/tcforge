package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "probe":
		return runProbe(args[1:])
	case "write":
		return runWrite(args[1:])
	case "batch":
		return runBatch(args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "--version", "version":
		fmt.Println("ltc2meta dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println(`ltc2meta turns audio LTC into video timecode metadata.

Usage:
  ltc2meta probe <input> [--json]
  ltc2meta write <input> --channel left|right|1|2 --fps <fps> --output <output.mov> [--drop-ltc-audio] [--overwrite] [--dry-run] [--json] [--verbose]
  ltc2meta batch <folder> --channel left|right|1|2 --fps <fps> --output-dir <folder> [--drop-ltc-audio] [--overwrite] [--dry-run] [--json] [--verbose]

Required external tools on PATH:
  ffmpeg, ffprobe, ltcdump`)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func requireInputPath(input string) error {
	if input == "" {
		return errors.New("input path is required")
	}
	info, err := os.Stat(input)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected a media file", input)
	}
	return nil
}
