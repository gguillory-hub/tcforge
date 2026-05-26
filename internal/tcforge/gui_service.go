package tcforge

import (
	"context"
	"path/filepath"
)

type ClipProbe struct {
	Input       string       `json:"input"`
	Output      string       `json:"output"`
	InferredFPS string       `json:"inferred_fps,omitempty"`
	Tools       []ToolStatus `json:"tools"`
	Probe       ProbeInfo    `json:"probe"`
	Summary     ProbeSummary `json:"summary"`
	Status      string       `json:"status"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Error       string       `json:"error,omitempty"`
	Suggestion  string       `json:"suggestion,omitempty"`
}

type GUIGlobalSettings struct {
	EditEnabled      bool
	Channel          string
	FPS              string
	Preserve         bool
	Overwrite        bool
	AllowFPSMismatch bool
	OutputDir        string
}

func ProbeClip(ctx context.Context, input, outputDir string) ClipProbe {
	result := ClipProbe{Input: input, Output: DefaultOutput(input, outputDir), Status: "failed"}
	fail := func(err error) ClipProbe {
		code, suggestion := appErrorFields(err)
		result.ErrorCode = code
		result.Error = appErrorSummary(err)
		result.Suggestion = suggestion
		return result
	}

	if err := requireInputPath(input); err != nil {
		return fail(err)
	}
	tools, err := checkRequiredTools()
	result.Tools = tools
	if err != nil {
		return fail(err)
	}
	probe, err := ffprobe(ctx, input)
	if err != nil {
		return fail(wrapProbeError(input, err))
	}
	result.Probe = probe
	result.Summary = summarizeProbe(probe)
	result.InferredFPS = inferredVideoFPS(probe)
	result.Status = "ok"
	return result
}

func FixClip(ctx context.Context, input string, settings GUIGlobalSettings) (WriteResult, error) {
	fps := ""
	channel := "auto"
	preserve := false
	if settings.EditEnabled {
		channel = coalesceString(settings.Channel, "auto")
		if settings.FPS != "auto" {
			fps = settings.FPS
		}
		preserve = settings.Preserve
	}
	if fps == "" {
		inferred, err := inferFixFPS(ctx, input)
		if err != nil {
			return failedWriteResult(input, DefaultOutput(input, settings.OutputDir), channel, err), err
		}
		fps = inferred
	}
	options := WriteOptions{
		Input:            input,
		Output:           DefaultOutput(input, settings.OutputDir),
		Channel:          channel,
		FPS:              fps,
		Clean:            !preserve,
		Overwrite:        settings.EditEnabled && settings.Overwrite,
		AllowFPSMismatch: settings.EditEnabled && settings.AllowFPSMismatch,
	}
	return writeOne(ctx, options)
}

func DefaultOutput(input, outputDir string) string {
	return outputForFixInput(input, "", outputDir)
}

func ListMediaFiles(dir string) ([]string, error) {
	return mediaFiles(dir)
}

func SupportedMedia(path string) bool {
	return isSupportedMedia(path)
}

func inferredVideoFPS(probe ProbeInfo) string {
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			return canonicalFPS(streamFPS(stream))
		}
	}
	return ""
}

func DisplayName(path string) string {
	return filepath.Base(path)
}
