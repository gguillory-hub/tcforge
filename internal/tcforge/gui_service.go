package tcforge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	GUIStatusReady              = "Ready"
	GUIStatusScanning           = "Scanning"
	GUIStatusProcessing         = "Processing"
	GUIStatusFixed              = "Fixed"
	GUIStatusNeedsAttention     = "Needs Attention"
	GUIStatusFailed             = "Failed"
	GUIStatusAlreadyHasTimecode = "Already Has Timecode"
	GUIStatusAlreadyProcessed   = "Already Processed"
	GUIStatusNoAudioLTCFound    = "No Audio LTC Found"
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

type ClipScan struct {
	ClipProbe
	LTCScan       *LTCScanResult `json:"ltc_scan,omitempty"`
	OutputExists  bool           `json:"output_exists"`
	Overwrite     bool           `json:"overwrite"`
	Warnings      []string       `json:"warnings,omitempty"`
	Display       ClipDisplay    `json:"display"`
	GUIStatus     string         `json:"gui_status"`
	TechnicalLog  string         `json:"technical_log,omitempty"`
	TCForgeTagged bool           `json:"tcforge_tagged"`
}

type ClipDisplay struct {
	Video         string `json:"video,omitempty"`
	Audio         string `json:"audio,omitempty"`
	DetectedLTC   string `json:"detected_ltc,omitempty"`
	StartTimecode string `json:"start_timecode,omitempty"`
	Output        string `json:"output,omitempty"`
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

type GUIProgressEvent struct {
	Input   string
	Stage   string
	Current int
	Total   int
	Percent float64
	Exact   bool
}

type GUIProgressFunc func(GUIProgressEvent)

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
	return FixClipWithProgress(ctx, input, settings, nil)
}

func FixClipWithProgress(ctx context.Context, input string, settings GUIGlobalSettings, progress GUIProgressFunc) (WriteResult, error) {
	emit := func(stage string, percent float64, exact bool) {
		if progress != nil {
			progress(GUIProgressEvent{Input: input, Stage: stage, Percent: percent, Exact: exact})
		}
	}
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
		emit("Detecting video FPS", 0.05, false)
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
		Progress: func(stage string, percent float64, exact bool) {
			emit(stage, percent, exact)
		},
	}
	return writeOne(ctx, options)
}

func ScanClip(ctx context.Context, input string, settings GUIGlobalSettings) ClipScan {
	probe := ProbeClip(ctx, input, settings.OutputDir)
	scan := ClipScan{
		ClipProbe:    probe,
		OutputExists: fileExists(probe.Output),
		Overwrite:    settings.EditEnabled && settings.Overwrite,
		GUIStatus:    GUIStatusReady,
	}
	if probe.Status != "ok" {
		scan.GUIStatus = classifyGUIError(probe.ErrorCode)
		scan.Display = clipDisplay(scan)
		scan.TechnicalLog = scanTechnicalLog(scan)
		return scan
	}

	scan.TCForgeTagged = hasTCForgeMetadata(probe.Probe)
	if scan.TCForgeTagged {
		scan.GUIStatus = GUIStatusAlreadyProcessed
		scan.Warnings = append(scan.Warnings, "Already Processed. This file already appears to contain TCForge timecode metadata.")
		scan.Display = clipDisplay(scan)
		scan.TechnicalLog = scanTechnicalLog(scan)
		return scan
	}
	if scan.OutputExists && !scan.Overwrite {
		scan.Warnings = append(scan.Warnings, fmt.Sprintf("Output already exists and overwrite is off: %s", probe.Output))
		if scan.GUIStatus == GUIStatusReady {
			scan.GUIStatus = GUIStatusNeedsAttention
		}
	}

	fps := scanFPS(probe, settings)
	if fps == "" {
		scan.Warnings = append(scan.Warnings, "Could not infer video FPS for LTC scan.")
		if scan.GUIStatus == GUIStatusReady {
			scan.GUIStatus = GUIStatusNeedsAttention
		}
		scan.Display = clipDisplay(scan)
		scan.TechnicalLog = scanTechnicalLog(scan)
		return scan
	}
	ltc, err := scanLTCChannels(ctx, input, fps, probe.Probe)
	if err != nil {
		code, suggestion := appErrorFields(err)
		scan.ErrorCode = code
		scan.Error = appErrorSummary(err)
		scan.Suggestion = suggestion
		scan.GUIStatus = classifyGUIError(code)
		scan.Display = clipDisplay(scan)
		scan.TechnicalLog = scanTechnicalLog(scan)
		return scan
	}
	scan.LTCScan = &ltc
	if ltc.SelectedChannel == "" {
		if scan.GUIStatus == GUIStatusReady {
			scan.GUIStatus = GUIStatusNoAudioLTCFound
		}
		scan.Warnings = append(scan.Warnings, fmt.Sprintf("No audio LTC found on %d audio channel(s).", audioChannelCount(probe.Probe)))
	}
	applyExistingTimecodeStatus(&scan)
	scan.Display = clipDisplay(scan)
	scan.TechnicalLog = scanTechnicalLog(scan)
	return scan
}

func applyExistingTimecodeStatus(scan *ClipScan) {
	if len(scan.Summary.ExistingTimecodes) == 0 {
		return
	}
	if scan.LTCScan != nil && scan.LTCScan.SelectedTimecode != "" {
		formatMismatchFound := false
		if formatMismatch := timecodeFormatMismatch(scan.Summary.ExistingTimecodes, scan.LTCScan.SelectedTimecode); formatMismatch != "" {
			scan.Warnings = append(scan.Warnings, formatMismatch)
			formatMismatchFound = true
			if scan.GUIStatus == GUIStatusReady || scan.GUIStatus == GUIStatusAlreadyHasTimecode {
				scan.GUIStatus = GUIStatusNeedsAttention
			}
		}
		if mismatch := timecodeMismatch(scan.Summary.ExistingTimecodes, scan.LTCScan.SelectedTimecode); mismatch != "" {
			scan.Warnings = append(scan.Warnings, "Audio LTC differs from existing camera timecode metadata. Fix will write the detected audio LTC timecode to the output file.")
			scan.Warnings = append(scan.Warnings, mismatch)
			if scan.GUIStatus == GUIStatusReady || scan.GUIStatus == GUIStatusAlreadyHasTimecode {
				scan.GUIStatus = GUIStatusNeedsAttention
			}
			return
		}
		if formatMismatchFound {
			return
		}
		scan.Warnings = append(scan.Warnings, "Existing timecode metadata matches detected audio LTC.")
		if scan.GUIStatus == GUIStatusReady {
			scan.GUIStatus = GUIStatusAlreadyHasTimecode
		}
		return
	}
	scan.Warnings = append(scan.Warnings, "This file already has timecode metadata.")
	if scan.GUIStatus == GUIStatusReady {
		scan.GUIStatus = GUIStatusAlreadyHasTimecode
	}
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

func ClassifyWriteResult(result WriteResult, err error) string {
	if err == nil && result.Status == "ok" {
		return GUIStatusFixed
	}
	return classifyGUIError(result.ErrorCode)
}

func HumanSummary(scan ClipScan) ClipDisplay {
	return clipDisplay(scan)
}

func BatchScanWarnings(scans []ClipScan) []string {
	existingFormats := map[string]bool{}
	ltcFormats := map[string]bool{}
	for _, scan := range scans {
		for _, tc := range scan.Summary.ExistingTimecodes {
			if format := timecodeFormat(tc.Value); format != timecodeFormatUnknown {
				existingFormats[format] = true
			}
		}
		if scan.LTCScan != nil && scan.LTCScan.SelectedTimecode != "" {
			if format := timecodeFormat(scan.LTCScan.SelectedTimecode); format != timecodeFormatUnknown {
				ltcFormats[format] = true
			}
		}
	}

	var warnings []string
	if existingFormats[timecodeFormatDrop] && existingFormats[timecodeFormatNonDrop] {
		warnings = append(warnings, "Scanned files contain mixed existing camera timecode formats: drop-frame and non-drop. Check camera settings before syncing.")
	}
	if existingFormats[timecodeFormatDrop] && ltcFormats[timecodeFormatNonDrop] {
		warnings = append(warnings, "Scanned files contain mixed timecode formats: existing camera metadata includes drop-frame, but decoded audio LTC includes non-drop. Check camera and timecode-box settings before syncing.")
	}
	if existingFormats[timecodeFormatNonDrop] && ltcFormats[timecodeFormatDrop] {
		warnings = append(warnings, "Scanned files contain mixed timecode formats: existing camera metadata includes non-drop, but decoded audio LTC includes drop-frame. Check camera and timecode-box settings before syncing.")
	}
	return warnings
}

func scanFPS(probe ClipProbe, settings GUIGlobalSettings) string {
	if settings.EditEnabled && settings.FPS != "" && settings.FPS != "auto" {
		return settings.FPS
	}
	return probe.InferredFPS
}

func classifyGUIError(code string) string {
	switch code {
	case "ltc_not_found":
		return GUIStatusNoAudioLTCFound
	case "command_canceled":
		return GUIStatusNeedsAttention
	case "output_exists", "fps_mismatch", "audio_channel_missing":
		return GUIStatusNeedsAttention
	case "":
		return GUIStatusFailed
	default:
		return GUIStatusFailed
	}
}

func hasTCForgeMetadata(probe ProbeInfo) bool {
	if hasTCForgeTag(probe.Format.Tags) {
		return true
	}
	for _, stream := range probe.Streams {
		if hasTCForgeTag(stream.Tags) {
			return true
		}
	}
	return false
}

func hasTCForgeTag(tags map[string]string) bool {
	for key, value := range tags {
		k := strings.ToLower(strings.TrimSpace(key))
		v := strings.ToLower(strings.TrimSpace(value))
		if k == "tcforge" && v != "" {
			return true
		}
		if strings.Contains(v, "tcforge") {
			return true
		}
	}
	return false
}

func clipDisplay(scan ClipScan) ClipDisplay {
	display := ClipDisplay{Output: filepath.Base(scan.Output)}
	if len(scan.Summary.Video) > 0 {
		v := scan.Summary.Video[0]
		res := friendlyResolution(v.Resolution)
		codec := friendlyCodec(v.Codec)
		fps := scan.InferredFPS
		if fps == "" {
			fps = canonicalFPS(v.FPS)
		}
		display.Video = strings.Join(nonEmptyStrings(res, fpsLabel(fps), codec), ", ")
	}
	display.Audio = audioDisplay(scan.Summary.Audio)
	if scan.LTCScan != nil && scan.LTCScan.SelectedChannel != "" {
		display.DetectedLTC = displayChannel(scan.LTCScan.SelectedChannel)
		display.StartTimecode = scan.LTCScan.SelectedTimecode
	} else if len(scan.Summary.ExistingTimecodes) > 0 {
		display.StartTimecode = scan.Summary.ExistingTimecodes[0].Value
	}
	return display
}

func scanTechnicalLog(scan ClipScan) string {
	var parts []string
	if b, err := json.MarshalIndent(scan.Summary, "", "  "); err == nil {
		parts = append(parts, "Probe summary:\n"+string(b))
	}
	if scan.LTCScan != nil {
		if b, err := json.MarshalIndent(scan.LTCScan, "", "  "); err == nil {
			parts = append(parts, "LTC scan:\n"+string(b))
		}
	}
	parts = append(parts, nonEmptyStrings("Status: "+scan.GUIStatus, "Output: "+scan.Output, strings.Join(scan.Warnings, "\n"))...)
	if scan.Error != "" {
		parts = append(parts, "Error: "+scan.Error)
	}
	if scan.Suggestion != "" {
		parts = append(parts, "Suggestion: "+scan.Suggestion)
	}
	return strings.Join(parts, "\n\n")
}

func friendlyResolution(res string) string {
	switch res {
	case "3840x2160", "4096x2160":
		return "4K UHD"
	case "1920x1080":
		return "1080p"
	case "1280x720":
		return "720p"
	default:
		return res
	}
}

func friendlyCodec(codec string) string {
	switch strings.ToLower(codec) {
	case "h264":
		return "H.264"
	case "hevc", "h265":
		return "H.265"
	case "prores":
		return "ProRes"
	default:
		return codec
	}
}

func fpsLabel(fps string) string {
	if fps == "" {
		return ""
	}
	return fps + " fps"
}

func channelLabel(channels int) string {
	if channels <= 0 {
		return ""
	}
	if channels == 1 {
		return "1 channel"
	}
	return fmt.Sprintf("%d channels", channels)
}

func audioDisplay(audio []AudioSummary) string {
	if len(audio) == 0 {
		return ""
	}
	if len(audio) == 1 {
		a := audio[0]
		return strings.Join(nonEmptyStrings(channelLabel(a.Channels), sampleRateLabel(a.SampleRate)), ", ")
	}
	totalChannels := 0
	sameSampleRate := audio[0].SampleRate
	for _, a := range audio {
		if a.Channels <= 0 {
			totalChannels++
		} else {
			totalChannels += a.Channels
		}
		if a.SampleRate != sameSampleRate {
			sameSampleRate = ""
		}
	}
	streamLabel := fmt.Sprintf("%d audio streams", len(audio))
	if totalChannels == len(audio) {
		streamLabel = fmt.Sprintf("%d mono audio streams", len(audio))
	}
	return strings.Join(nonEmptyStrings(streamLabel, channelLabel(totalChannels), sampleRateLabel(sameSampleRate)), ", ")
}

func sampleRateLabel(sampleRate string) string {
	if sampleRate == "" {
		return ""
	}
	value, err := strconv.Atoi(sampleRate)
	if err != nil {
		return sampleRate + " Hz"
	}
	if value%1000 == 0 {
		return fmt.Sprintf("%d kHz", value/1000)
	}
	return sampleRate + " Hz"
}

func titleWord(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + strings.ToLower(value[1:])
}

func nonEmptyStrings(values ...string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func OutputExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
