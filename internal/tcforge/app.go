package tcforge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "probe":
		return runProbe(args[1:])
	case "fix":
		return runFix(args[1:])
	case "verify":
		return runVerify(args[1:])
	case "write":
		return runWrite(args[1:])
	case "batch":
		return runBatch(args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "--version", "version":
		fmt.Println(VersionString())
		return nil
	default:
		if !strings.HasPrefix(args[0], "-") {
			return runFix(args)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println(`tcforge turns audio LTC into video timecode metadata.

Usage:
  tcforge probe <input> [--scan-ltc --fps <fps>] [--json]
  tcforge verify <input> [--json]
  tcforge fix <input...> [--fps <fps>] [--output-dir <folder>] [--channel auto|left|right|1|2] [--preserve] [--overwrite] [--allow-fps-mismatch] [--dry-run] [--json] [--verbose]
  tcforge fix --manifest <jobs.json> [--output-dir <folder>] [--dry-run] [--json] [--verbose]
  tcforge <input...> [--fps <fps>] [--output-dir <folder>]
  tcforge write <input> --channel auto|left|right|1|2 --fps <fps> --output <output.mov> [--clean] [--drop-ltc-audio] [--overwrite] [--allow-fps-mismatch] [--dry-run] [--json] [--verbose]
  tcforge batch <folder> --channel auto|left|right|1|2 --fps <fps> --output-dir <folder> [--clean] [--drop-ltc-audio] [--overwrite] [--dry-run] [--json] [--verbose]

Required external tools on PATH:
  ffmpeg, ffprobe, ltcdump

Packaged releases also look for bundled tools in:
  tools\ beside the tcforge executable`)
}

func VersionString() string {
	parts := []string{"tcforge", version}
	if commit != "" {
		parts = append(parts, "commit="+commit)
	}
	if date != "" {
		parts = append(parts, "built="+date)
	}
	return strings.Join(parts, " ")
}

func versionString() string {
	return VersionString()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func requireInputPath(input string) error {
	if input == "" {
		return appError("missing_input", "Input path is required.", "Pass a video file path, for example: tcforge fix C1315.MP4 --fps 29.97", errors.New("input path is required"))
	}
	info, err := os.Stat(input)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return appError("input_not_found", fmt.Sprintf("Input file not found: %s", input), "Check the file path and make sure the drive or card is mounted.", err)
		}
		return appError("input_unreadable", fmt.Sprintf("Could not access input file: %s", input), "Check file permissions and make sure another app is not locking the file.", err)
	}
	if info.IsDir() {
		return appError("input_is_directory", fmt.Sprintf("%s is a directory, expected a media file.", input), "Pass a specific video file or use the batch command for folders.", nil)
	}
	return nil
}
