package main

import (
	"context"
	"flag"
	"fmt"
)

type VerifyResult struct {
	Input    string        `json:"input"`
	Timecode string        `json:"timecode,omitempty"`
	FPS      string        `json:"fps,omitempty"`
	Checks   []VerifyCheck `json:"checks"`
	Status   string        `json:"status"`
	Error    string        `json:"error,omitempty"`
}

type VerifyCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(flagsFirst(args, map[string]bool{})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("verify requires exactly one input path")
	}
	input := fs.Arg(0)
	result, err := verifyOne(context.Background(), input)
	if err != nil {
		result.Status = "failed"
		result.Error = appErrorSummary(err)
		if *jsonOut {
			_ = printJSON(result)
		}
		return err
	}
	if *jsonOut {
		_ = printJSON(result)
	} else {
		printVerifyResult(result)
	}
	if result.Status == "failed" {
		return appError("verify_failed", "Verification failed.", "Review the failed checks above, then run probe for more detail.", nil)
	}
	return nil
}

func verifyOne(ctx context.Context, input string) (VerifyResult, error) {
	result := VerifyResult{Input: input, Status: "failed"}
	if err := requireInputPath(input); err != nil {
		return result, err
	}
	if _, err := resolveTool("ffprobe"); err != nil {
		return result, appError("missing_tools", "Missing required external tool: ffprobe.", "Install ffprobe and make sure it is available on PATH.", err)
	}
	probe, err := ffprobe(ctx, input)
	if err != nil {
		return result, wrapProbeError(input, err)
	}
	return analyzeVerification(input, probe), nil
}

func analyzeVerification(input string, probe ProbeInfo) VerifyResult {
	result := VerifyResult{Input: input, Status: "ok"}
	summary := summarizeProbe(probe)
	result.Timecode = firstTimecode(probe)
	if len(summary.Video) > 0 {
		result.FPS = canonicalFPS(summary.Video[0].FPS)
		if result.FPS == "" {
			result.FPS = summary.Video[0].FPS
		}
	}

	result.addCheck("ffprobe_readable", "ok", "readable by ffprobe", "")

	if len(summary.Video) > 0 {
		result.addCheck("video_present", "ok", "video stream present", fmt.Sprintf("stream #%d", summary.Video[0].Index))
	} else {
		result.addCheck("video_present", "failed", "no video stream found", "")
	}

	if result.Timecode != "" {
		result.addCheck("timecode_metadata", "ok", "timecode metadata found", result.Timecode)
	} else {
		result.addCheck("timecode_metadata", "failed", "no timecode metadata found", "")
	}

	if tc := tmcdTimecode(summary); tc != "" {
		result.addCheck("tmcd_track", "ok", "tmcd/data timecode track present", tc)
	} else {
		result.addCheck("tmcd_track", "failed", "no tmcd/data timecode track found", "")
	}

	if result.FPS != "" {
		result.addCheck("fps", "ok", "video fps detected", result.FPS)
	} else {
		result.addCheck("fps", "warn", "video fps was not detected", "")
	}

	if len(summary.Audio) == 0 {
		result.addCheck("audio_removed", "ok", "no audio streams present", "")
	} else {
		result.addCheck("audio_removed", "warn", "audio stream still present", fmt.Sprintf("%d audio stream(s)", len(summary.Audio)))
	}

	result.Status = verificationStatus(result.Checks)
	return result
}

func (r *VerifyResult) addCheck(name, status, message, value string) {
	r.Checks = append(r.Checks, VerifyCheck{Name: name, Status: status, Message: message, Value: value})
}

func tmcdTimecode(summary ProbeSummary) string {
	for _, data := range summary.Data {
		if data.Timecode != "" {
			return data.Timecode
		}
	}
	return ""
}

func verificationStatus(checks []VerifyCheck) string {
	status := "ok"
	for _, check := range checks {
		switch check.Status {
		case "failed":
			return "failed"
		case "warn":
			status = "warning"
		}
	}
	return status
}

func printVerifyResult(result VerifyResult) {
	fmt.Println("Input:", result.Input)
	for _, check := range result.Checks {
		mark := "OK  "
		if check.Status == "warn" {
			mark = "WARN"
		}
		if check.Status == "failed" {
			mark = "FAIL"
		}
		line := fmt.Sprintf("%s %s", mark, check.Message)
		if check.Value != "" {
			line += ": " + check.Value
		}
		fmt.Println(line)
	}
	fmt.Println("Status:", result.Status)
}
