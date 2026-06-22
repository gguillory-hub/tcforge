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
		themeChoice:      normalizeThemeChoice(a.Preferences().StringWithFallback("theme", themeChoiceProfessionalDark)),
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
	state.rowBoxes.Add(emptyListState())

	w.SetContent(state.buildUI())
	w.ShowAndRun()
}

func (s *guiState) buildUI() fyne.CanvasObject {
	addFile := widget.NewButtonWithIcon("Add Files", theme.FileIcon(), func() {
		paths, err := openMediaFiles()
		if err != nil {
			s.handleNativeDialogError(err)
			return
		}
		s.addPaths(paths)
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
	s.processButton.Importance = widget.HighImportance
	settings := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		s.showSettings()
	})
	settings.Importance = widget.LowImportance
	selectAll := widget.NewButton("Select All", func() {
		s.setAllSelected(true)
	})
	selectAll.Importance = widget.LowImportance
	selectNone := widget.NewButton("Select None", func() {
		s.setAllSelected(false)
	})
	selectNone.Importance = widget.LowImportance

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
	channel := widget.NewSelect([]string{"auto", "left", "right", "3", "4"}, func(value string) {
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
	version := widget.NewLabelWithStyle(guiVersionLabel(), fyne.TextAlignTrailing, fyne.TextStyle{})
	version.Importance = widget.LowImportance
	topRow := container.NewBorder(nil, nil, nil, version, toolbar)
	progress := container.NewVBox(s.progressLabel, s.progressBar, s.progressInfinite)
	header := container.NewVBox(topRow, outputRow, advanced, progress, widget.NewSeparator())
	list := container.NewVScroll(s.rowBoxes)
	right := container.NewVScroll(s.detailPanel)
	right.SetMinSize(fyne.NewSize(420, 0))
	body := container.NewHSplit(list, right)
	body.SetOffset(0.72)
	return container.NewBorder(header, nil, nil, nil, body)
}

func guiVersionLabel() string {
	parts := strings.Fields(tcforge.VersionString())
	if len(parts) < 2 {
		return tcforge.VersionString()
	}
	label := parts[0] + " " + parts[1]
	for _, part := range parts[2:] {
		if commit, ok := strings.CutPrefix(part, "commit="); ok && commit != "" {
			if len(commit) > 7 {
				commit = commit[:7]
			}
			label += " (" + commit + ")"
			break
		}
	}
	return label
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
				row.Scan.Display.DetectedLTC = tcforge.DisplayChannel(result.SelectedChannel)
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
	themeSelect := widget.NewSelect([]string{"Professional Dark", "System", "Light", "Fyne Dark"}, func(value string) {
		if value != "" {
			s.applyTheme(themeChoiceFromLabel(value))
		}
	})
	themeSelect.SetSelected(themeLabel(s.themeChoice))
	content := container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(tcforge.VersionString()),
		container.NewBorder(nil, nil, widget.NewLabel("Theme"), nil, themeSelect),
	)
	fynedialog.ShowCustom("Settings", "Close", content, s.window)
}

func (s *guiState) applyTheme(choice string) {
	s.themeChoice = normalizeThemeChoice(choice)
	s.app.Preferences().SetString("theme", s.themeChoice)
	switch s.themeChoice {
	case themeChoiceProfessionalDark:
		s.app.Settings().SetTheme(newProfessionalDarkTheme())
	case themeChoiceSystem:
		s.app.Settings().SetTheme(theme.DefaultTheme())
	case themeChoiceLight:
		s.app.Settings().SetTheme(theme.LightTheme())
	case themeChoiceFyneDark:
		s.app.Settings().SetTheme(theme.DarkTheme())
	default:
		s.themeChoice = themeChoiceProfessionalDark
		s.app.Settings().SetTheme(newProfessionalDarkTheme())
	}
	s.refreshRows()
	s.refreshDetails()
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
		if len(rows) == 0 {
			s.rowBoxes.Add(emptyListState())
			s.rowBoxes.Refresh()
			return
		}
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
	name.SizeName = theme.SizeNameSubHeadingText
	path := widget.NewLabel(shortPath(row.Path))
	path.Truncation = fyne.TextTruncateEllipsis
	path.Importance = widget.LowImportance

	timecode := widget.NewLabelWithStyle(labelLine("Start Timecode", row.Scan.Display.StartTimecode), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	if row.Scan.Display.StartTimecode == "" {
		timecode.Hide()
	}

	lines := []string{
		labelLine("Video", row.Scan.Display.Video),
		labelLine("Audio", row.Scan.Display.Audio),
		labelLine("Detected LTC", row.Scan.Display.DetectedLTC),
		labelLine("Output", row.Scan.Display.Output),
	}
	if row.Stage != "" {
		lines = append(lines, labelLine("Progress", row.Stage))
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
	showDetails.Importance = widget.LowImportance
	openOutput.Importance = widget.LowImportance

	header := container.NewBorder(nil, nil, container.NewHBox(check, statusIcon, statusText), container.NewHBox(showDetails, openOutput), name)
	objects := []fyne.CanvasObject{header, timecode, summary}
	if notice := s.rowNotice(row); notice != nil {
		objects = append(objects, notice)
	}
	objects = append(objects, path)
	content := container.NewVBox(objects...)
	bg := canvas.NewRectangle(s.statusTint(row.Status))
	return container.NewVBox(container.NewStack(bg, container.NewPadded(content)))
}

func (s *guiState) refreshDetails() {
	fyne.Do(func() {
		s.detailPanel.Objects = nil
		s.mu.Lock()
		row := s.selectedDetail
		s.mu.Unlock()
		s.detailPanel.Add(widget.NewLabelWithStyle("Selected File Details", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if row == nil {
			s.detailPanel.Add(widget.NewLabel("Select a file to see scan results, output plan, warnings, and technical details."))
			s.detailPanel.Refresh()
			return
		}
		file := widget.NewLabelWithStyle(tcforge.DisplayName(row.Path), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		file.SizeName = theme.SizeNameSubHeadingText
		s.detailPanel.Add(file)
		s.detailPanel.Add(s.statusPill(row.Status))
		s.detailPanel.Add(sectionText("Detected media", strings.Join(nonEmpty(
			labelLine("Video", row.Scan.Display.Video),
			labelLine("Audio", row.Scan.Display.Audio),
		), "\n")))
		s.detailPanel.Add(sectionText("Detected LTC", strings.Join(nonEmpty(
			labelLine("Channel", row.Scan.Display.DetectedLTC),
			labelLine("Start", row.Scan.Display.StartTimecode),
		), "\n")))
		s.detailPanel.Add(sectionText("Output plan", row.Scan.Output))
		s.detailPanel.Add(s.warningMessages(row.Scan.Warnings))
		if row.Error != "" || row.Suggestion != "" {
			s.detailPanel.Add(s.errorMessage(strings.Join(nonEmpty(row.Error, row.Suggestion), "\n")))
		}
		s.detailPanel.Add(technicalDetails(strings.Join(nonEmpty(row.Scan.TechnicalLog, commandLog(row.Result), row.Error, row.Suggestion), "\n\n")))
		s.detailPanel.Refresh()
	})
}

func (s *guiState) showDetails(row *clipRow) {
	content := container.NewVScroll(container.NewVBox(
		sectionText("Detected media", strings.Join(nonEmpty(labelLine("Video", row.Scan.Display.Video), labelLine("Audio", row.Scan.Display.Audio)), "\n")),
		sectionText("Detected LTC", strings.Join(nonEmpty(labelLine("Channel", row.Scan.Display.DetectedLTC), labelLine("Start", row.Scan.Display.StartTimecode)), "\n")),
		sectionText("Output plan", row.Scan.Output),
		s.warningMessages(row.Scan.Warnings),
		s.errorMessage(strings.Join(nonEmpty(row.Error, row.Suggestion), "\n")),
		technicalDetails(strings.Join(nonEmpty(row.Scan.TechnicalLog, commandLog(row.Result), row.Error, row.Suggestion), "\n\n")),
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

func (s *guiState) rowNotice(row *clipRow) fyne.CanvasObject {
	message := rowMessage(row)
	if message == "" {
		return nil
	}
	switch tcforge.GUIStatusTone(row.Status) {
	case tcforge.GUIStatusToneError:
		return s.messageBox("Error", message, tcforge.GUIStatusToneError)
	case tcforge.GUIStatusToneWarning:
		return s.messageBox("Warning", message, tcforge.GUIStatusToneWarning)
	default:
		if row.Status == tcforge.GUIStatusAlreadyProcessed {
			return s.messageBox("Notice", message, tcforge.GUIStatusToneActive)
		}
		return s.messageBox("Notice", message, tcforge.GUIStatusToneNeutral)
	}
}

func (s *guiState) warningMessages(messages []string) fyne.CanvasObject {
	if len(nonEmpty(messages...)) == 0 {
		return sectionText("Warnings", "")
	}
	return s.messageBox("Warnings", strings.Join(nonEmpty(messages...), "\n"), tcforge.GUIStatusToneWarning)
}

func (s *guiState) errorMessage(message string) fyne.CanvasObject {
	if strings.TrimSpace(message) == "" {
		return sectionText("Errors", "")
	}
	return s.messageBox("Errors", message, tcforge.GUIStatusToneError)
}

func (s *guiState) messageBox(title, body, tone string) fyne.CanvasObject {
	bg := canvas.NewRectangle(s.messageColor(tone))
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	bodyLabel := widget.NewLabel(body)
	bodyLabel.Wrapping = fyne.TextWrapWord
	icon := widget.NewIcon(messageIcon(tone))
	return container.NewStack(bg, container.NewPadded(container.NewBorder(nil, nil, icon, nil, container.NewVBox(titleLabel, bodyLabel))))
}

func (s *guiState) messageColor(tone string) color.Color {
	if s.lightMode() {
		switch tone {
		case tcforge.GUIStatusToneError:
			return color.NRGBA{R: 0xfa, G: 0xdb, B: 0xdb, A: 0xff}
		case tcforge.GUIStatusToneWarning:
			return color.NRGBA{R: 0xff, G: 0xee, B: 0xc2, A: 0xff}
		default:
			return color.NRGBA{R: 0xe4, G: 0xf0, B: 0xff, A: 0xff}
		}
	}
	switch tone {
	case tcforge.GUIStatusToneError:
		return color.NRGBA{R: 0x45, G: 0x25, B: 0x28, A: 0xff}
	case tcforge.GUIStatusToneWarning:
		return color.NRGBA{R: 0x4a, G: 0x3b, B: 0x20, A: 0xff}
	default:
		return color.NRGBA{R: 0x24, G: 0x30, B: 0x3d, A: 0xff}
	}
}

func messageIcon(tone string) fyne.Resource {
	switch tone {
	case tcforge.GUIStatusToneError:
		return theme.ErrorIcon()
	case tcforge.GUIStatusToneWarning:
		return theme.WarningIcon()
	default:
		return theme.InfoIcon()
	}
}

func emptyListState() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("Add media files to begin", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	body := widget.NewLabel("Use Add Files or Add Folder, then Scan Files to inspect timecode before writing.")
	body.Alignment = fyne.TextAlignCenter
	body.Wrapping = fyne.TextWrapWord
	body.Importance = widget.LowImportance
	return container.NewCenter(container.NewPadded(container.NewVBox(title, body)))
}

func (s *guiState) statusPill(status string) fyne.CanvasObject {
	bg := canvas.NewRectangle(s.statusTint(status))
	label := widget.NewLabelWithStyle(status, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewStack(bg, container.NewPadded(container.NewHBox(widget.NewIcon(statusResource(status)), label)))
}

func (s *guiState) statusTint(status string) color.Color {
	if s.lightMode() {
		switch tcforge.GUIStatusTone(status) {
		case tcforge.GUIStatusToneSuccess:
			return color.NRGBA{R: 0xe6, G: 0xf4, B: 0xec, A: 0xff}
		case tcforge.GUIStatusToneWarning:
			return color.NRGBA{R: 0xff, G: 0xf3, B: 0xd6, A: 0xff}
		case tcforge.GUIStatusToneError:
			return color.NRGBA{R: 0xfa, G: 0xe3, B: 0xe3, A: 0xff}
		case tcforge.GUIStatusToneActive:
			return color.NRGBA{R: 0xe4, G: 0xf0, B: 0xff, A: 0xff}
		default:
			if status == tcforge.GUIStatusAlreadyProcessed {
				return color.NRGBA{R: 0xe8, G: 0xee, B: 0xf6, A: 0xff}
			}
			return color.NRGBA{R: 0xf5, G: 0xf7, B: 0xfa, A: 0xff}
		}
	}
	switch tcforge.GUIStatusTone(status) {
	case tcforge.GUIStatusToneSuccess:
		return color.NRGBA{R: 0x23, G: 0x35, B: 0x2b, A: 0xff}
	case tcforge.GUIStatusToneWarning:
		return color.NRGBA{R: 0x37, G: 0x31, B: 0x22, A: 0xff}
	case tcforge.GUIStatusToneError:
		return color.NRGBA{R: 0x36, G: 0x25, B: 0x27, A: 0xff}
	case tcforge.GUIStatusToneActive:
		return color.NRGBA{R: 0x24, G: 0x30, B: 0x3d, A: 0xff}
	default:
		if status == tcforge.GUIStatusAlreadyProcessed {
			return color.NRGBA{R: 0x28, G: 0x2f, B: 0x38, A: 0xff}
		}
		return color.NRGBA{R: 0x26, G: 0x29, B: 0x2d, A: 0xff}
	}
}

func (s *guiState) lightMode() bool {
	if s.themeChoice == themeChoiceLight {
		return true
	}
	return s.themeChoice == themeChoiceSystem && s.app.Settings().ThemeVariant() == theme.VariantLight
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

func technicalDetails(body string) fyne.CanvasObject {
	if strings.TrimSpace(body) == "" {
		body = "No technical details yet."
	}
	log := widget.NewLabelWithStyle(body, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	log.Wrapping = fyne.TextWrapWord
	log.Selectable = true
	item := widget.NewAccordionItem("Technical Details", container.NewPadded(log))
	accordion := widget.NewAccordion(item)
	accordion.CloseAll()
	return accordion
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
