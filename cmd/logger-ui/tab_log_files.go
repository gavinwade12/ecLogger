package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	labelGraph  = "Graph"
	labelDelete = "Delete"
	// labelLogging marks the file being logged to right now.
	labelLogging = "Logging"
)

// logFileTimeFormat is how a log file's modified time is shown.
const logFileTimeFormat = "2006-01-02 15:04"

// LogFilesTab lists the CSV files in the log directory, to graph or delete.
type LogFilesTab struct {
	app *App

	list       *fyne.Container
	refreshBtn *widget.Button
	dirLabel   *widget.Label
	status     *hint
}

func NewLogFilesTab(app *App) *LogFilesTab {
	tab := &LogFilesTab{
		app:      app,
		list:     container.New(newListLayout(theme.Padding())),
		dirLabel: widget.NewLabel(""),
		status:   newHint(""),
	}
	tab.refreshBtn = widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), tab.refresh)
	tab.dirLabel.Truncation = fyne.TextTruncateEllipsis
	tab.status.Wrapping = fyne.TextWrapWord
	tab.status.segment.Style.Alignment = fyne.TextAlignLeading
	tab.status.Hide()
	return tab
}

func (t *LogFilesTab) Container() fyne.CanvasObject {
	header := container.NewPadded(container.NewBorder(nil, nil,
		t.refreshBtn, nil, t.dirLabel))
	top := container.NewVBox(header, container.NewPadded(t.status))
	return container.NewBorder(top, nil, nil, nil,
		container.NewVScroll(container.NewPadded(t.list)))
}

func (t *LogFilesTab) setStatus(s string, isError bool) {
	if s == "" {
		t.status.Hide()
		return
	}
	if isError {
		t.status.SetError(s)
	} else {
		t.status.SetText(s)
	}
	t.status.Show()
}

// logFile is a log file in the log directory.
type logFile struct {
	name    string
	path    string
	modTime time.Time
	size    int64
}

// listLogFiles returns the CSV files in dir, newest first. A directory
// that doesn't exist has no files.
func listLogFiles(dir string) ([]logFile, error) {
	// Check what's at dir before reading it: whether ReadDir on a file
	// counts as not-exist varies by OS and Go version, and a file there
	// shouldn't look like an empty directory.
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("it's a file, not a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []logFile
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, logFile{
			name:    e.Name(),
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}
	sort.SliceStable(files, func(i, j int) bool {
		if !files[i].modTime.Equal(files[j].modTime) {
			return files[i].modTime.After(files[j].modTime)
		}
		return files[i].name > files[j].name
	})
	return files, nil
}

// refresh lists the log directory again. A directory listing is cheap, so
// it's read on the UI goroutine.
func (t *LogFilesTab) refresh() {
	dir := t.app.config.LogDirectory
	t.dirLabel.SetText(dir)
	t.list.RemoveAll()

	files, err := listLogFiles(dir)
	switch {
	case err != nil:
		logger.Debug(err.Error())
		t.setStatus("Couldn't read "+dir+": "+err.Error(), true)
	case len(files) == 0:
		t.setStatus("No log files in "+dir+". Log To File on the Logging tab writes them there.", false)
	default:
		t.setStatus("", false)
	}

	current := t.app.LoggingTab.CurrentLogFile()
	widths := logFileColumnWidths()
	for _, f := range files {
		t.list.Objects = append(t.list.Objects, t.newRow(widths, f, f.path == current))
	}
	t.list.Refresh()
}

// logFileColumnWidths returns the widths of the modified time, size and
// buttons columns.
func logFileColumnWidths() []float32 {
	textSize := theme.TextSize()
	modified := fyne.MeasureText("0000-00-00 00:00", textSize, fyne.TextStyle{}).Width
	size := fyne.MeasureText("1023.9 MB", textSize, fyne.TextStyle{}).Width
	buttons := container.NewHBox(
		widget.NewButtonWithIcon(labelGraph, graphIcon(), nil),
		widget.NewButtonWithIcon(labelDelete, theme.DeleteIcon(), nil)).MinSize().Width
	return []float32{modified, size, buttons}
}

// The log file row columns: its name, modified time, size and buttons.
const logFileNameColumn = 0

func (t *LogFilesTab) newRow(widths []float32, f logFile, logging bool) *columnsRow {
	name := widget.NewLabel(f.name)
	name.Truncation = fyne.TextTruncateEllipsis
	var nameCell fyne.CanvasObject = name
	if logging {
		badge := newWrappedText(labelLogging, theme.SizeNameText, theme.ColorNameError)
		badge.style.Bold = true
		nameCell = container.NewBorder(nil, nil, nil, badge, name)
	}
	modified := newWrappedText(f.modTime.Format(logFileTimeFormat), theme.SizeNameText, colorNameSecondaryText)
	size := newWrappedText(humanSize(f.size), theme.SizeNameText, colorNameSecondaryText)
	size.alignment = fyne.TextAlignTrailing

	graphBtn := widget.NewButtonWithIcon(labelGraph, graphIcon(), func() { t.graph(f) })
	deleteBtn := widget.NewButtonWithIcon(labelDelete, theme.DeleteIcon(), func() { t.confirmDelete(f) })
	if logging {
		deleteBtn.Disable()
	}
	row := newColumnsRow(widths, logFileNameColumn, colorNameTile,
		nameCell, modified, size, container.NewHBox(graphBtn, deleteBtn))
	row.verticalInset = theme.SizeNamePadding
	return row
}

func graphIcon() fyne.Resource { return theme.VisibilityIcon() }

func (t *LogFilesTab) graph(f logFile) {
	t.app.GraphingTab.OpenFile(f.path)
	t.app.SelectTab(TabGraphing)
}

// confirmDelete asks before deleting f.
func (t *LogFilesTab) confirmDelete(f logFile) {
	if t.app.win == nil {
		return
	}
	d := dialog.NewConfirm("Delete log file",
		"Delete "+f.name+"? This can't be undone.",
		func(ok bool) {
			if ok {
				t.delete(f)
			}
		}, t.app.win)
	d.SetConfirmText(labelDelete)
	d.SetDismissText("Cancel")
	d.Show()
}

func (t *LogFilesTab) delete(f logFile) {
	err := os.Remove(f.path)
	t.refresh()
	if err != nil {
		logger.Debug(err.Error())
		t.setStatus("Couldn't delete "+f.name+": "+err.Error(), true)
	}
}

// humanSize returns a size in bytes as, for example, "1.2 MB".
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	size, suffix := float64(n), ""
	for _, s := range []string{"KB", "MB", "GB", "TB"} {
		size /= unit
		suffix = s
		if size < unit {
			break
		}
	}
	return fmt.Sprintf("%.1f %s", size, suffix)
}
