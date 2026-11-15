package main

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/internal/logging"
)

// defaultGraphedColumns is how many of a log's first columns are charted
// when it's opened.
const defaultGraphedColumns = 4

// graphingPanelWidth is the width of the parameter list, which leaves most
// of an 800 pixel wide window to the chart.
const graphingPanelWidth = 210

// GraphingTab charts the columns of a log file.
type GraphingTab struct {
	app *App

	openBtn   *widget.Button
	paramsBtn *widget.Button
	fileName  *widget.Label

	// panel lists the log's columns to choose from, and is hidden until a
	// log is open. It can also be hidden to give the chart the full width.
	panel     *fyne.Container
	panelOpen bool
	checkList *fyne.Container
	checks    []*widget.Check
	chart     *laneChart
	// header and view are refreshed to lay out again when something in
	// them is shown or hidden
	buttons, header, view *fyne.Container
	chartView             fyne.CanvasObject
	status                *hint
	statusView            fyne.CanvasObject

	log *logging.Log
	// loads numbers the files being opened so a slow load that finishes
	// after a newer one is dropped.
	loads atomic.Uint64
}

func NewGraphingTab(app *App) *GraphingTab {
	t := &GraphingTab{
		app:       app,
		openBtn:   widget.NewButtonWithIcon("Open…", theme.FolderOpenIcon(), nil),
		paramsBtn: widget.NewButtonWithIcon("Parameters", theme.ListIcon(), nil),
		fileName:  widget.NewLabel(""),
		checkList: container.NewVBox(),
		chart:     newLaneChart(),
		status:    newHint(""),
		panelOpen: true,
	}
	t.fileName.Truncation = fyne.TextTruncateEllipsis
	t.openBtn.OnTapped = t.showOpenDialog
	t.paramsBtn.OnTapped = func() {
		t.panelOpen = !t.panelOpen
		t.updateView()
	}
	t.paramsBtn.Hide()
	t.status.Wrapping = fyne.TextWrapWord

	t.panel = container.New(fixedWidthLayout{graphingPanelWidth},
		container.NewBorder(nil, nil, nil, widget.NewSeparator(), container.NewVScroll(t.checkList)))
	t.panel.Hide()
	t.chartView = container.NewVScroll(t.chart)
	t.chartView.Hide()
	t.statusView = container.NewCenter(container.NewGridWrap(fyne.NewSize(360, t.status.MinSize().Height*3), t.status))
	t.setHint("Open a log file to graph it. Log files are written from the Logging tab.")
	return t
}

func (t *GraphingTab) Container() fyne.CanvasObject {
	if t.view != nil {
		return t.view
	}
	t.buttons = container.NewHBox(t.openBtn, t.paramsBtn)
	t.header = container.NewBorder(nil, nil, t.buttons, nil, t.fileName)
	body := container.NewStack(t.chartView, t.statusView)
	t.view = container.NewBorder(container.NewPadded(t.header), nil, t.panel, nil, body)
	return t.view
}

func (t *GraphingTab) showOpenDialog() {
	win := t.app.win
	if win == nil {
		return
	}
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			t.setError("Couldn't open the file: " + err.Error())
			return
		}
		if r == nil {
			return
		}
		path := r.URI().Path()
		r.Close()
		t.OpenFile(path)
	}, win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	if dir := t.app.config.LogDirectory; dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			if u, err := storage.ListerForURI(storage.NewFileURI(dir)); err == nil {
				d.SetLocation(u)
			}
		}
	}
	d.Resize(win.Canvas().Size())
	d.Show()
}

// OpenFile shows the log file at path, replacing any file already open. The
// file is read on another goroutine, so this returns right away.
func (t *GraphingTab) OpenFile(path string) {
	id := t.loads.Add(1)
	t.fileName.SetText(filepath.Base(path))
	t.setHint("Loading " + filepath.Base(path) + "…")
	t.setLog(nil)

	go func() {
		l, err := readLogFile(path)
		fyne.Do(func() {
			if t.loads.Load() != id {
				return
			}
			if err != nil {
				logger.Debug(err.Error())
				t.setError("Couldn't open " + filepath.Base(path) + ": " + err.Error())
				return
			}
			t.setLog(l)
		})
	}()
}

func readLogFile(path string) (*logging.Log, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return logging.ReadCSV(f)
}

// setLog shows l, or nothing when it's nil.
func (t *GraphingTab) setLog(l *logging.Log) {
	t.log = l
	t.checks = nil
	t.checkList.Objects = nil
	if l != nil {
		for i, c := range l.Columns {
			check := widget.NewCheck(paramTitle(c.Name, c.Unit), nil)
			check.Checked = i < defaultGraphedColumns
			check.OnChanged = func(bool) { t.updateChart() }
			t.checks = append(t.checks, check)
			t.checkList.Add(check)
		}
		if len(l.Times) > 0 {
			from, to := l.Times[0], l.Times[len(l.Times)-1]
			if !to.After(from) {
				to = from.Add(time.Second)
			}
			t.chart.SetTimeRange(from, to)
			t.chart.SetTimeAxis(from, elapsedLabels(from))
		}
	}
	t.checkList.Refresh()
	t.updateChart()
}

// updateChart charts the checked columns, in column order.
func (t *GraphingTab) updateChart() {
	if t.log == nil {
		t.updateView()
		return
	}
	var columns []int
	for i, c := range t.checks {
		if c.Checked {
			columns = append(columns, i)
		}
	}
	t.chart.SetLanes(lanesFromLog(t.log, columns))

	switch {
	case len(t.log.Times) == 0:
		t.status.SetText("This log has no rows")
	case len(columns) == 0:
		t.status.SetText("Choose parameters to graph")
	default:
		t.status.SetText("")
	}
	t.updateView()
}

// updateView shows the chart, or the status that explains why there isn't
// one, and the parameter list when there's a log to choose from.
func (t *GraphingTab) updateView() {
	hasLog := t.log != nil
	charted := hasLog && len(t.log.Times) > 0 && t.status.Text() == ""
	setShown(t.chartView, charted)
	setShown(t.statusView, !charted)
	setShown(t.paramsBtn, hasLog)
	setShown(t.panel, hasLog && t.panelOpen)
	if t.view != nil {
		t.buttons.Refresh()
		t.header.Refresh()
		t.view.Refresh()
	}
}

func setShown(o fyne.CanvasObject, shown bool) {
	if shown {
		o.Show()
	} else {
		o.Hide()
	}
}

func (t *GraphingTab) setHint(s string) {
	t.status.SetText(s)
	t.updateView()
}

func (t *GraphingTab) setError(s string) {
	t.status.SetError(s)
	t.updateView()
}

// fixedWidthLayout gives its content a set width and all the height there is.
type fixedWidthLayout struct{ width float32 }

func (l fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(l.width, size.Height))
	}
}

func (l fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objects {
		h = max(h, o.MinSize().Height)
	}
	return fyne.NewSize(l.width, h)
}
