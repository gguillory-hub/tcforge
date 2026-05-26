//go:build gui

package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynedialog "fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	nativedialog "github.com/sqweek/dialog"

	"github.com/gguillory-hub/tcforge/internal/tcforge"
)

type clipRow struct {
	Path          string
	Selected      bool
	Status        string
	Scan          tcforge.ClipScan
	Result        tcforge.WriteResult
	Error         string
	Suggestion    string
	Progress      float64
	ProgressExact bool
	Stage         string
}

type guiState struct {
	app              fyne.App
	mu               sync.Mutex
	rows             []*clipRow
	selectedDetail   *clipRow
	rowBoxes         *fyne.Container
	window           fyne.Window
	outputDir        string
	editEnabled      bool
	channel          string
	fps              string
	preserve         bool
	overwrite        bool
	allowMismatch    bool
	themeChoice      string
	scanButton       *widget.Button
	processButton    *widget.Button
	outputEntry      *widget.Entry
	progressBar      *widget.ProgressBar
	progressInfinite *widget.ProgressBarInfinite
	progressLabel    *widget.Label
	detailPanel      *fyne.Container
	mismatchWarning  *fyne.Container
	advancedFields   []fyne.Disableable
}

func main() {
	a := app.NewWithID("com.tcforge.gui")
	w := a.NewWindow("tcforge")
	w.Resize(fyne.NewSize(1260, 780))

	state := &guiState{
		app:              a,
		window:           w,
		channel:          "auto",
		fps:              "auto",
		themeChoice:      a.Preferences().StringWithFallback("theme", "system"),
		rowBoxes:         container.NewVBox(),
		outputEntry:      widget.NewEntry(),
		progressBar:      widget.NewProgressBar(),
		progressInfinite: widget.NewProgressBarInfinite(),
		progressLabel:    widget.NewLabel("Ready"),
		detailPanel:      container.NewVBox(),
	}
	state.applyTheme(state.themeChoice)
	state.outputEntry.SetPlaceHolder("Same folder as source")
	state.outputEntry.Disable()
	state.progressBar.Hide()
	state.progressInfinite.Hide()
	state.refreshDetails()

	w.SetContent(state.buildUI())
	w.ShowAndRun()
}

func (s *guiState) buildUI() fyne.CanvasObject {
	addFile := widget.NewButtonWithIcon("Add Files", theme.FileIcon(), func() {
		path, err := nativedialog.File().Title("Add Media File").Filter("Media files", "mp4", "mov", "m4v", "mxf").Load()
		if err != nil {
			s.handleNativeDialogError(err)
			return
		}
		s.addPaths([]string{path})
	})
	addFolder := widget.NewButtonWithIcon("Add Folder", theme.FolderOpenIcon(), func() {
		path, err := nativedialog.Directory().Title("Add Media Folder").Browse()
		if err != nil {
			s.handleNativeDialogError(err)
			return
		}
		files, err := tcforge.ListMediaFiles(path)
		if err != nil {
			fynedialog.ShowError(err, s.window)
			return
		}
		s.addPaths(files)
	})
	s.scanButton = widget.NewButtonWithIcon("Scan Files", theme.SearchIcon(), func() {
		s.scanSelected()
	})
	s.processButton = widget.NewButtonWithIcon("Fix Selected", theme.MediaPlayIcon(), func() {
		s.processSelected()
	})
	settings := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		s.showSettings()
	})
	selectAll := widget.NewButton("Select All", func() {
		s.setAllSelected(true)
	})
	selectNone := widget.NewButton("Select None", func() {
		s.setAllSelected(false)
	})

	outputChoose := widget.NewButtonWithIcon("Choose", theme.FolderOpenIcon(), func() {
		path, err := nativedialog.Directory().Title("Choose Output Folder").Browse()
		if err != nil {
			s.handleNativeDialogError(err)
			return
		}
		s.setOutputDir(path)
	})
	outputClear := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		s.setOutputDir("")
	})
	outputClear.Importance = widget.LowImportance
	outputRow := container.NewBorder(nil, nil, widget.NewLabel("Output Folder:"), container.NewHBox(outputChoose, outputClear), s.outputEntry)

	edit := widget.NewCheck("Edit settings", func(enabled bool) {
		s.editEnabled = enabled
		s.updateAdvancedEnabled()
	})
	channel := widget.NewSelect([]string{"auto", "left", "right"}, func(value string) {
		s.channel = value
	})
	channel.SetSelected("auto")
	fps := widget.NewSelect([]string{"auto", "23.976", "24", "25", "29.97", "30", "47.952", "48", "50", "59.94", "60"}, func(value string) {
		s.fps = value
	})
	fps.SetSelected("auto")
	preserve := widget.NewCheck("Keep original audio and data streams", func(value bool) {
		s.preserve = value
	})
	overwrite := widget.NewCheck("Overwrite outputs", func(value bool) {
		s.overwrite = value
	})
	allowMismatch := widget.NewCheck("Advanced: allow timecode FPS to differ from video FPS", func(value bool) {
		s.allowMismatch = value
		s.updateMismatchWarning()
	})
	s.mismatchWarning = warningBanner("Timecode FPS mismatch is enabled")
	s.mismatchWarning.Hide()
	s.advancedFields = []fyne.Disableable{channel, fps, preserve, overwrite, allowMismatch}
	s.updateAdvancedEnabled()
	advanced := container.NewVBox(
		container.NewHBox(edit, widget.NewLabel("Channel"), channel, widget.NewLabel("FPS"), fps),
		container.NewHBox(preserve, overwrite, allowMismatch),
		s.mismatchWarning,
	)

	toolbar := container.NewHBox(addFile, addFolder, s.scanButton, s.processButton, settings, selectAll, selectNone)
	progress := container.NewVBox(s.progressLabel, s.progressBar, s.progressInfinite)
	header := container.NewVBox(toolbar, outputRow, advanced, progress, widget.NewSeparator())
	list := container.NewVScroll(s.rowBoxes)
	right := container.NewVScroll(s.detailPanel)
	right.SetMinSize(fyne.NewSize(360, 0))
	body := container.NewBorder(nil, nil, nil, right, list)
	return container.NewBorder(header, nil, nil, nil, body)
}

func (s *guiState) addPaths(paths []string) {
	for _, path := range paths {
		if path == "" || !tcforge.SupportedMedia(path) || s.hasPath(path) {
			continue
		}
		row := &clipRow{
			Path:     path,
			Selected: true,
			Status:   tcforge.GUIStatusReady,
			Scan: tcforge.ClipScan{
				ClipProbe: tcforge.ClipProbe{Input: path, Output: tcforge.DefaultOutput(path, s.outputDir)},
				GUIStatus: tcforge.GUIStatusReady,
				Display:   tcforge.ClipDisplay{Output: filepath.Base(tcforge.DefaultOutput(path, s.outputDir))},
			},
		}
		s.mu.Lock()
		s.rows = append(s.rows, row)
		if s.selectedDetail == nil {
			s.selectedDetail = row
		}
		s.mu.Unlock()
	}
	s.refreshRows()
	s.refreshDetails()
}

func (s *guiState) hasPath(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	clean := filepath.Clean(path)
	for _, row := range s.rows {
		if filepath.Clean(row.Path) == clean {
			return true
		}
	}
	return false
}

func (s *guiState) scanSelected() {
	selected := s.selectedRows()
	if len(selected) == 0 {
		fynedialog.ShowInformation("No clips selected", "Select one or more clips to scan.", s.window)
		return
	}
	s.setBusy(true)
	go func() {
		counts := map[string]int{}
		for i, row := range selected {
			s.setProgress(fmt.Sprintf("Scanning %d of %d: %s", i+1, len(selected), tcforge.DisplayName(row.Path)), batchPercent(i, len(selected)), false)
			s.setRowStatus(row, tcforge.GUIStatusScanning, "Scanning", 0, false)
			scan := tcforge.ScanClip(context.Background(), row.Path, s.settings())
			s.mu.Lock()
			row.Scan = scan
			row.Status = scan.GUIStatus
			row.Error = scan.Error
			row.Suggestion = scan.Suggestion
			row.Stage = ""
			row.Progress = 0
			counts[row.Status]++
			s.mu.Unlock()
			s.refreshRows()
			s.refreshDetails()
		}
		s.finishBatch("Scan complete", counts)
	}()
}

func (s *guiState) processSelected() {
	selected := s.selectedRows()
	if len(selected) == 0 {
		fynedialog.ShowInformation("No clips selected", "Select one or more clips to fix.", s.window)
		return
	}
	s.setBusy(true)
	go func() {
		counts := map[string]int{}
		for i, row := range selected {
			s.setProgress(fmt.Sprintf("Fixing %d of %d: %s", i+1, len(selected), tcforge.DisplayName(row.Path)), batchPercent(i, len(selected)), false)
			if row.Scan.Status != "ok" && row.Scan.LTCScan == nil {
				scan := tcforge.ScanClip(context.Background(), row.Path, s.settings())
				s.mu.Lock()
				row.Scan = scan
				row.Status = scan.GUIStatus
				row.Error = scan.Error
				row.Suggestion = scan.Suggestion
				s.mu.Unlock()
				s.refreshRows()
				if scan.GUIStatus == tcforge.GUIStatusAlreadyProcessed || scan.GUIStatus == tcforge.GUIStatusNoAudioLTCFound || scan.GUIStatus == tcforge.GUIStatusFailed {
					counts[scan.GUIStatus]++
					continue
				}
			}
			s.setRowStatus(row, tcforge.GUIStatusProcessing, "Processing", 0, false)
			result, err := tcforge.FixClipWithProgress(context.Background(), row.Path, s.settings(), func(event tcforge.GUIProgressEvent) {
				s.setRowStatus(row, tcforge.GUIStatusProcessing, event.Stage, event.Percent, event.Exact)
				if event.Exact {
					s.setProgress(fmt.Sprintf("%s: %s", tcforge.DisplayName(row.Path), event.Stage), event.Percent, true)
				} else {
					s.setProgress(fmt.Sprintf("%s: %s", tcforge.DisplayName(row.Path), event.Stage), batchPercent(i, len(selected)), false)
				}
			})
			status := tcforge.ClassifyWriteResult(result, err)
			s.mu.Lock()
			row.Result = result
			row.Status = status
			row.Error = result.Error
			row.Suggestion = result.Suggestion
			if err == nil {
				row.Scan.TCForgeTagged = true
				row.Scan.Display.StartTimecode = result.DecodedStartTC
				row.Scan.Display.DetectedLTC = titleWord(result.SelectedChannel) + " channel"
				row.Scan.Display.Output = filepath.Base(result.Output)
				row.Scan.Output = result.Output
				row.Error = ""
				row.Suggestion = ""
			}
			row.Stage = ""
			row.Progress = 0
			counts[status]++
			s.mu.Unlock()
			s.refreshRows()
			s.refreshDetails()
		}
		s.finishBatch("Fix complete", counts)
	}()
}

func (s *guiState) finishBatch(title string, counts map[string]int) {
	s.setBusy(false)
	s.setProgress("Ready", 0, false)
	body := summaryText(counts)
	s.app.SendNotification(fyne.NewNotification(title, body))
	fyne.Do(func() {
		fynedialog.ShowInformation(title, body, s.window)
	})
}

func (s *guiState) showSettings() {
	themeSelect := widget.NewSelect([]string{"system", "light", "dark"}, func(value string) {
		if value != "" {
			s.applyTheme(value)
		}
	})
	themeSelect.SetSelected(s.themeChoice)
	content := container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Theme"), nil, themeSelect),
	)
	fynedialog.ShowCustom("Settings", "Close", content, s.window)
}

func (s *guiState) applyTheme(choice string) {
	s.themeChoice = choice
	s.app.Preferences().SetString("theme", choice)
	switch choice {
	case "light":
		s.app.Settings().SetTheme(theme.LightTheme())
	case "dark":
		s.app.Settings().SetTheme(theme.DarkTheme())
	default:
		s.themeChoice = "system"
		s.app.Settings().SetTheme(theme.DefaultTheme())
	}
}

func (s *guiState) handleNativeDialogError(err error) {
	if err == nil || err == nativedialog.ErrCancelled {
		return
	}
	fynedialog.ShowError(err, s.window)
}

func (s *guiState) selectedRows() []*clipRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected []*clipRow
	for _, row := range s.rows {
		if row.Selected {
			selected = append(selected, row)
		}
	}
	return selected
}

func (s *guiState) setBusy(busy bool) {
	fyne.Do(func() {
		if busy {
			s.scanButton.Disable()
			s.processButton.Disable()
			return
		}
		s.scanButton.Enable()
		s.processButton.Enable()
	})
}

func (s *guiState) setRowStatus(row *clipRow, status, stage string, progress float64, exact bool) {
	s.mu.Lock()
	row.Status = status
	row.Stage = stage
	row.Progress = progress
	row.ProgressExact = exact
	row.Error = ""
	row.Suggestion = ""
	s.mu.Unlock()
	s.refreshRows()
	s.refreshDetails()
}

func (s *guiState) setProgress(label string, value float64, exact bool) {
	fyne.Do(func() {
		s.progressLabel.SetText(label)
		if exact {
			s.progressInfinite.Stop()
			s.progressInfinite.Hide()
			s.progressBar.Show()
			s.progressBar.SetValue(value)
		} else if strings.TrimSpace(label) == "Ready" {
			s.progressInfinite.Stop()
			s.progressInfinite.Hide()
			s.progressBar.Hide()
		} else {
			s.progressBar.Show()
			s.progressBar.SetValue(value)
			s.progressInfinite.Show()
			s.progressInfinite.Start()
		}
	})
}

func (s *guiState) settings() tcforge.GUIGlobalSettings {
	return tcforge.GUIGlobalSettings{
		EditEnabled:      s.editEnabled,
		Channel:          s.channel,
		FPS:              s.fps,
		Preserve:         s.preserve,
		Overwrite:        s.overwrite,
		AllowFPSMismatch: s.allowMismatch,
		OutputDir:        s.outputDir,
	}
}

func (s *guiState) setAllSelected(selected bool) {
	s.mu.Lock()
	for _, row := range s.rows {
		row.Selected = selected
	}
	s.mu.Unlock()
	s.refreshRows()
}

func (s *guiState) setOutputDir(path string) {
	s.outputDir = path
	if path == "" {
		s.outputEntry.SetText("")
	} else {
		s.outputEntry.SetText(path)
	}
	s.mu.Lock()
	for _, row := range s.rows {
		output := tcforge.DefaultOutput(row.Path, s.outputDir)
		row.Scan.Output = output
		row.Scan.Display.Output = filepath.Base(output)
	}
	s.mu.Unlock()
	s.refreshRows()
	s.refreshDetails()
}

func (s *guiState) updateAdvancedEnabled() {
	for _, field := range s.advancedFields {
		if s.editEnabled {
			field.Enable()
		} else {
			field.Disable()
		}
	}
	s.updateMismatchWarning()
}

func (s *guiState) updateMismatchWarning() {
	if s.mismatchWarning == nil {
		return
	}
	if s.editEnabled && s.allowMismatch {
		s.mismatchWarning.Show()
	} else {
		s.mismatchWarning.Hide()
	}
}

func (s *guiState) refreshRows() {
	fyne.Do(func() {
		s.rowBoxes.Objects = nil
		s.mu.Lock()
		rows := append([]*clipRow(nil), s.rows...)
		s.mu.Unlock()
		for _, row := range rows {
			s.rowBoxes.Add(s.rowWidget(row))
		}
		s.rowBoxes.Refresh()
	})
}

func (s *guiState) rowWidget(row *clipRow) fyne.CanvasObject {
	check := widget.NewCheck("", func(value bool) {
		s.mu.Lock()
		row.Selected = value
		if value {
			s.selectedDetail = row
		}
		s.mu.Unlock()
		s.refreshDetails()
	})
	check.SetChecked(row.Selected)

	statusIcon := widget.NewIcon(statusResource(row.Status))
	statusText := widget.NewLabelWithStyle(row.Status, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name := widget.NewLabelWithStyle(tcforge.DisplayName(row.Path), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	path := widget.NewLabel(shortPath(row.Path))
	path.Truncation = fyne.TextTruncateEllipsis

	lines := []string{
		labelLine("Video", row.Scan.Display.Video),
		labelLine("Audio", row.Scan.Display.Audio),
		labelLine("Detected LTC", row.Scan.Display.DetectedLTC),
		labelLine("Start Timecode", row.Scan.Display.StartTimecode),
		labelLine("Output", row.Scan.Display.Output),
	}
	if row.Stage != "" {
		lines = append(lines, labelLine("Progress", row.Stage))
	}
	message := rowMessage(row)
	if message != "" {
		lines = append(lines, message)
	}
	summary := widget.NewLabel(strings.Join(nonEmpty(lines...), "\n"))
	summary.Wrapping = fyne.TextWrapWord

	showDetails := widget.NewButton("Show Details", func() {
		s.mu.Lock()
		s.selectedDetail = row
		s.mu.Unlock()
		s.refreshDetails()
		s.showDetails(row)
	})
	openOutput := widget.NewButton("Open Output", func() {
		s.openOutput(row)
	})
	if !outputAvailable(row) {
		openOutput.Disable()
	}

	header := container.NewBorder(nil, nil, container.NewHBox(check, statusIcon, statusText), container.NewHBox(showDetails, openOutput), name)
	card := widget.NewCard("", "", container.NewVBox(header, path, summary))
	return card
}

func (s *guiState) refreshDetails() {
	fyne.Do(func() {
		s.detailPanel.Objects = nil
		s.mu.Lock()
		row := s.selectedDetail
		s.mu.Unlock()
		s.detailPanel.Add(widget.NewLabelWithStyle("Selected File Details", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if row == nil {
			s.detailPanel.Add(widget.NewLabel("Select a file to see details."))
			s.detailPanel.Refresh()
			return
		}
		s.detailPanel.Add(widget.NewLabel(tcforge.DisplayName(row.Path)))
		s.detailPanel.Add(sectionText("Detected media", strings.Join(nonEmpty(
			labelLine("Video", row.Scan.Display.Video),
			labelLine("Audio", row.Scan.Display.Audio),
		), "\n")))
		s.detailPanel.Add(sectionText("Detected LTC", strings.Join(nonEmpty(
			labelLine("Channel", row.Scan.Display.DetectedLTC),
			labelLine("Start", row.Scan.Display.StartTimecode),
		), "\n")))
		s.detailPanel.Add(sectionText("Output plan", row.Scan.Output))
		s.detailPanel.Add(sectionText("Warnings", strings.Join(row.Scan.Warnings, "\n")))
		s.detailPanel.Add(sectionText("Technical log", strings.Join(nonEmpty(row.Scan.TechnicalLog, commandLog(row.Result), row.Error, row.Suggestion), "\n\n")))
		s.detailPanel.Refresh()
	})
}

func (s *guiState) showDetails(row *clipRow) {
	content := container.NewVScroll(container.NewVBox(
		sectionText("Detected media", strings.Join(nonEmpty(labelLine("Video", row.Scan.Display.Video), labelLine("Audio", row.Scan.Display.Audio)), "\n")),
		sectionText("Detected LTC", strings.Join(nonEmpty(labelLine("Channel", row.Scan.Display.DetectedLTC), labelLine("Start", row.Scan.Display.StartTimecode)), "\n")),
		sectionText("Output plan", row.Scan.Output),
		sectionText("Warnings", strings.Join(row.Scan.Warnings, "\n")),
		sectionText("Technical log", strings.Join(nonEmpty(row.Scan.TechnicalLog, commandLog(row.Result), row.Error, row.Suggestion), "\n\n")),
	))
	content.SetMinSize(fyne.NewSize(760, 520))
	fynedialog.ShowCustom(tcforge.DisplayName(row.Path), "Close", content, s.window)
}

func (s *guiState) openOutput(row *clipRow) {
	output := rowOutput(row)
	if output == "" {
		return
	}
	if err := revealFile(output); err != nil {
		fynedialog.ShowError(err, s.window)
	}
}

func warningBanner(text string) *fyne.Container {
	bg := canvas.NewRectangle(color.NRGBA{R: 245, G: 190, B: 70, A: 90})
	label := widget.NewLabel(text)
	return container.NewStack(bg, container.NewPadded(label))
}

func statusResource(status string) fyne.Resource {
	switch status {
	case tcforge.GUIStatusFixed:
		return theme.ConfirmIcon()
	case tcforge.GUIStatusNeedsAttention, tcforge.GUIStatusAlreadyHasTimecode, tcforge.GUIStatusAlreadyProcessed:
		return theme.WarningIcon()
	case tcforge.GUIStatusFailed, tcforge.GUIStatusNoAudioLTCFound:
		return theme.ErrorIcon()
	case tcforge.GUIStatusScanning:
		return theme.SearchIcon()
	case tcforge.GUIStatusProcessing:
		return theme.MediaPlayIcon()
	default:
		return theme.InfoIcon()
	}
}

func sectionText(title, body string) fyne.CanvasObject {
	if strings.TrimSpace(body) == "" {
		body = "None"
	}
	label := widget.NewLabel(body)
	label.Wrapping = fyne.TextWrapWord
	return container.NewVBox(widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), label)
}

func labelLine(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return label + ": " + value
}

func rowMessage(row *clipRow) string {
	switch row.Status {
	case tcforge.GUIStatusAlreadyProcessed:
		return "Already Processed. This file already appears to contain TCForge timecode metadata."
	case tcforge.GUIStatusAlreadyHasTimecode:
		return "This file already has timecode metadata."
	case tcforge.GUIStatusNoAudioLTCFound:
		return "No audio LTC found."
	}
	return strings.Join(nonEmpty(row.Error, row.Suggestion, strings.Join(row.Scan.Warnings, " ")), " ")
}

func rowOutput(row *clipRow) string {
	if row.Result.Output != "" {
		return row.Result.Output
	}
	return row.Scan.Output
}

func outputAvailable(row *clipRow) bool {
	output := rowOutput(row)
	if output == "" {
		return false
	}
	info, err := os.Stat(output)
	return err == nil && !info.IsDir()
}

func revealFile(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

func commandLog(result tcforge.WriteResult) string {
	if len(result.Commands) == 0 {
		return ""
	}
	var lines []string
	for _, cmd := range result.Commands {
		lines = append(lines, cmd.Program+" "+strings.Join(cmd.Args, " "))
	}
	return "Commands:\n" + strings.Join(lines, "\n")
}

func shortPath(path string) string {
	dir := filepath.Dir(path)
	if len(dir) > 80 {
		return "..." + dir[len(dir)-77:] + string(filepath.Separator) + filepath.Base(path)
	}
	return path
}

func batchPercent(index, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(index) / float64(total)
}

func summaryText(counts map[string]int) string {
	order := []string{
		tcforge.GUIStatusFixed,
		tcforge.GUIStatusFailed,
		tcforge.GUIStatusNeedsAttention,
		tcforge.GUIStatusAlreadyHasTimecode,
		tcforge.GUIStatusAlreadyProcessed,
		tcforge.GUIStatusNoAudioLTCFound,
	}
	var lines []string
	for _, status := range order {
		if counts[status] > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d", status, counts[status]))
		}
	}
	if len(lines) == 0 {
		return "No files were changed."
	}
	return strings.Join(lines, "\n")
}

func titleWord(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + strings.ToLower(value[1:])
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}
