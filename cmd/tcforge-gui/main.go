//go:build gui

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	fynedialog "fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	nativedialog "github.com/sqweek/dialog"

	"github.com/gguillory-hub/tcforge/internal/tcforge"
)

type clipRow struct {
	Path        string
	Selected    bool
	Status      string
	StatusColor fyne.Resource
	Probe       tcforge.ClipProbe
	Result      tcforge.WriteResult
	Error       string
	Suggestion  string
}

type guiState struct {
	app            fyne.App
	mu             sync.Mutex
	rows           []*clipRow
	rowBoxes       *fyne.Container
	window         fyne.Window
	outputDir      string
	editEnabled    bool
	channel        string
	fps            string
	preserve       bool
	overwrite      bool
	allowMismatch  bool
	themeChoice    string
	processButton  *widget.Button
	outputEntry    *widget.Entry
	advancedFields []fyne.Disableable
}

func main() {
	a := app.NewWithID("com.tcforge.gui")
	w := a.NewWindow("tcforge")
	w.Resize(fyne.NewSize(1180, 760))

	state := &guiState{
		app:         a,
		window:      w,
		channel:     "auto",
		fps:         "auto",
		themeChoice: a.Preferences().StringWithFallback("theme", "system"),
		rowBoxes:    container.NewVBox(),
		outputEntry: widget.NewEntry(),
	}
	state.applyTheme(state.themeChoice)
	state.outputEntry.SetPlaceHolder("Same folder as each source clip")
	state.outputEntry.Disable()

	content := state.buildUI()
	w.SetContent(content)
	w.ShowAndRun()
}

func (s *guiState) buildUI() fyne.CanvasObject {
	addFile := widget.NewButtonWithIcon("Add File", theme.FileIcon(), func() {
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
	settings := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		s.showSettings()
	})
	selectAll := widget.NewButton("Select All", func() {
		s.setAllSelected(true)
	})
	selectNone := widget.NewButton("Select None", func() {
		s.setAllSelected(false)
	})
	s.processButton = widget.NewButtonWithIcon("Fix Selected", theme.MediaPlayIcon(), func() {
		s.processSelected()
	})

	outputChoose := widget.NewButtonWithIcon("Output Folder", theme.FolderOpenIcon(), func() {
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
	outputRow := container.NewBorder(nil, nil, widget.NewLabel("Output:"), container.NewHBox(outputChoose, outputClear), s.outputEntry)

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
	preserve := widget.NewCheck("Preserve streams", func(value bool) {
		s.preserve = value
	})
	overwrite := widget.NewCheck("Overwrite outputs", func(value bool) {
		s.overwrite = value
	})
	allowMismatch := widget.NewCheck("Allow FPS mismatch", func(value bool) {
		s.allowMismatch = value
	})
	s.advancedFields = []fyne.Disableable{channel, fps, preserve, overwrite, allowMismatch}
	s.updateAdvancedEnabled()
	advanced := container.NewHBox(edit, widget.NewLabel("Channel"), channel, widget.NewLabel("FPS"), fps, preserve, overwrite, allowMismatch)

	toolbar := container.NewHBox(addFile, addFolder, selectAll, selectNone, s.processButton, settings)
	header := container.NewVBox(toolbar, outputRow, advanced, widget.NewSeparator())
	list := container.NewVScroll(s.rowBoxes)
	return container.NewBorder(header, nil, nil, nil, list)
}

func (s *guiState) addPaths(paths []string) {
	for _, path := range paths {
		if path == "" || !tcforge.SupportedMedia(path) {
			continue
		}
		if s.hasPath(path) {
			continue
		}
		row := &clipRow{Path: path, Selected: true, Status: "Probing", StatusColor: theme.SearchIcon()}
		s.mu.Lock()
		s.rows = append(s.rows, row)
		s.mu.Unlock()
		s.refreshRows()
		go s.probe(row)
	}
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

func (s *guiState) probe(row *clipRow) {
	probe := tcforge.ProbeClip(context.Background(), row.Path, s.outputDir)
	s.mu.Lock()
	row.Probe = probe
	row.Status = "Ready"
	row.StatusColor = theme.ConfirmIcon()
	row.Error = ""
	row.Suggestion = ""
	if probe.Status != "ok" {
		row.Status = "Probe failed"
		row.StatusColor = theme.ErrorIcon()
		row.Error = probe.Error
		row.Suggestion = probe.Suggestion
	}
	s.mu.Unlock()
	s.refreshRows()
}

func (s *guiState) processSelected() {
	selected := s.selectedRows()
	if len(selected) == 0 {
		fynedialog.ShowInformation("No clips selected", "Select one or more clips to fix.", s.window)
		return
	}
	s.processButton.Disable()
	go func() {
		for _, row := range selected {
			s.setRowProcessing(row)
			result, err := tcforge.FixClip(context.Background(), row.Path, s.settings())
			s.mu.Lock()
			row.Result = result
			if err != nil {
				row.Status = "Failed"
				row.StatusColor = theme.ErrorIcon()
				row.Error = result.Error
				row.Suggestion = result.Suggestion
			} else {
				row.Status = "Fixed"
				row.StatusColor = theme.ConfirmIcon()
				row.Error = ""
				row.Suggestion = ""
			}
			s.mu.Unlock()
			s.refreshRows()
		}
		fyne.Do(func() {
			s.processButton.Enable()
		})
	}()
}

func (s *guiState) showSettings() {
	themeSelect := widget.NewSelect([]string{"system", "light", "dark"}, func(value string) {
		if value == "" {
			return
		}
		s.applyTheme(value)
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

func (s *guiState) setRowProcessing(row *clipRow) {
	s.mu.Lock()
	row.Status = "Processing"
	row.StatusColor = theme.MediaPlayIcon()
	row.Error = ""
	row.Suggestion = ""
	s.mu.Unlock()
	s.refreshRows()
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
		row.Probe.Output = tcforge.DefaultOutput(row.Path, s.outputDir)
	}
	s.mu.Unlock()
	s.refreshRows()
}

func (s *guiState) updateAdvancedEnabled() {
	for _, field := range s.advancedFields {
		if s.editEnabled {
			field.Enable()
		} else {
			field.Disable()
		}
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
		s.mu.Unlock()
	})
	check.SetChecked(row.Selected)

	status := widget.NewIcon(row.StatusColor)
	name := widget.NewLabelWithStyle(tcforge.DisplayName(row.Path), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	path := widget.NewLabel(row.Path)
	path.Truncation = fyne.TextTruncateEllipsis

	meta := widget.NewLabel(s.metadataText(row))
	meta.Wrapping = fyne.TextWrapWord
	result := widget.NewLabel(s.resultText(row))
	result.Wrapping = fyne.TextWrapWord

	statusText := widget.NewLabel(row.Status)
	statusText.Alignment = fyne.TextAlignCenter
	left := container.NewHBox(check, status, statusText)
	body := container.NewVBox(name, path, meta, result)
	return container.NewBorder(nil, widget.NewSeparator(), left, nil, body)
}

func (s *guiState) metadataText(row *clipRow) string {
	if row.Probe.Status != "ok" {
		if row.Error != "" {
			return row.Error
		}
		return "Waiting for probe data"
	}
	video := ""
	if len(row.Probe.Summary.Video) > 0 {
		v := row.Probe.Summary.Video[0]
		video = strings.TrimSpace(fmt.Sprintf("%s %s fps=%s", v.Codec, v.Resolution, row.Probe.InferredFPS))
	}
	audio := ""
	if len(row.Probe.Summary.Audio) > 0 {
		a := row.Probe.Summary.Audio[0]
		audio = strings.TrimSpace(fmt.Sprintf("audio=%s %dch %sHz", a.Codec, a.Channels, a.SampleRate))
	}
	duration := ""
	if row.Probe.Summary.Duration != "" {
		duration = "duration=" + row.Probe.Summary.Duration
	}
	return strings.Join(nonEmpty(video, audio, duration, "output="+row.Probe.Output), "  |  ")
}

func (s *guiState) resultText(row *clipRow) string {
	if row.Status == "Fixed" {
		return fmt.Sprintf("timecode=%s  channel=%s  output=%s", row.Result.DecodedStartTC, row.Result.SelectedChannel, row.Result.Output)
	}
	if row.Status == "Failed" || row.Status == "Probe failed" {
		return strings.Join(nonEmpty(row.Error, row.Suggestion), "  ")
	}
	return ""
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
