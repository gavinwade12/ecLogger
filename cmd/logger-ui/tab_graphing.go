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
	// liveBtn closes the open file: back to the live graph when connected,
	// or to nothing when not.
	liveBtn  *widget.Button
	fileName *widget.Label
	// zoomInBtn, zoomOutBtn and fitBtn change the time the chart shows,
	// and are shown with it. Fit zooms out to the whole log, or, when a
	// live chart has stopped following the present, resumes following.
	zoomInBtn, zoomOutBtn, fitBtn *widget.Button

	// panel lists the log's columns to choose from, and is hidden until a
	// log is open. It can also be hidden to give the chart the full width.
	panel     *fyne.Container
	panelOpen bool
	checkList *fyne.Container
	checks    []*widget.Check
	chart     *laneChart
	// header and view are refreshed to lay out again when something in
	// them is shown or hidden
	buttons, navButtons, header, view *fyne.Container
	chartView                         *container.Scroll
	status                            *hint
	statusView                        fyne.CanvasObject
	chooseParamsBtn                   *widget.Button
	// charted is whether the chart is shown rather than the status, and
	// needsParams whether the status offers to choose parameters
	charted, needsParams bool

	// fileMode is set from when a file is opened until it's closed, even
	// if the file is still loading or couldn't be read.
	fileMode bool
	log      *logging.Log
	// loads numbers the files being opened so a slow load that finishes
	// after a newer one is dropped.
	loads atomic.Uint64

	live
}

func NewGraphingTab(app *App) *GraphingTab {
	t := &GraphingTab{
		app:        app,
		openBtn:    widget.NewButtonWithIcon("Open…", theme.FolderOpenIcon(), nil),
		paramsBtn:  widget.NewButtonWithIcon("Parameters", theme.ListIcon(), nil),
		liveBtn:    widget.NewButtonWithIcon("Live", theme.MediaPlayIcon(), nil),
		zoomInBtn:  widget.NewButtonWithIcon("", theme.ZoomInIcon(), nil),
		zoomOutBtn: widget.NewButtonWithIcon("", theme.ZoomOutIcon(), nil),
		fitBtn:     widget.NewButtonWithIcon("", theme.ZoomFitIcon(), nil),
		fileName:   widget.NewLabel(""),
		checkList:  container.NewVBox(),
		chart:      newLaneChart(),
		status:     newHint(""),
		panelOpen:  true,
	}
	t.initLive()
	t.fileName.Truncation = fyne.TextTruncateEllipsis
	t.openBtn.OnTapped = t.showOpenDialog
	t.paramsBtn.OnTapped = func() {
		t.panelOpen = !t.panelOpen
		t.updateView()
	}
	t.paramsBtn.Hide()
	t.liveBtn.Hide()
	t.liveBtn.OnTapped = t.closeFile
	t.navButtons = container.NewHBox(t.zoomInBtn, t.zoomOutBtn, t.fitBtn)
	t.zoomInBtn.OnTapped = func() { t.zoomButton(1 / buttonZoom) }
	t.zoomOutBtn.OnTapped = func() { t.zoomButton(buttonZoom) }
	t.fitBtn.OnTapped = t.fitView
	t.chooseParamsBtn = widget.NewButton("Choose Parameters", func() { app.SelectTab(TabParameters) })
	t.chooseParamsBtn.Hide()
	t.status.Wrapping = fyne.TextWrapWord

	t.panel = container.New(fixedWidthLayout{graphingPanelWidth},
		container.NewBorder(nil, nil, nil, widget.NewSeparator(), container.NewVScroll(t.checkList)))
	t.panel.Hide()
	t.chartView = container.NewVScroll(t.chart)
	t.chartView.Hide()
	t.chart.SetScroller(t.chartView)
	t.chart.SetBounds(t.chartBounds)
	t.chart.OnNavigate(t.pauseLive)
	t.statusView = container.NewCenter(container.NewVBox(
		container.NewGridWrap(fyne.NewSize(360, t.status.MinSize().Height*3), t.status),
		container.NewCenter(t.chooseParamsBtn)))
	t.setHint(disconnectedHint)
	return t
}

func (t *GraphingTab) Container() fyne.CanvasObject {
	if t.view != nil {
		return t.view
	}
	t.buttons = container.NewHBox(t.openBtn, t.paramsBtn, t.liveBtn)
	t.header = container.NewBorder(nil, nil, t.buttons, t.navButtons, t.fileName)
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
	t.fileMode = true
	t.paused = false
	t.syncTicker()
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
	t.checkIDs = nil
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

// closeFile closes the open file, to show the live graph when connected, or
// the hint to open a file when not.
func (t *GraphingTab) closeFile() {
	t.loads.Add(1) // drop a file still loading
	t.fileMode = false
	t.paused = false
	t.log = nil
	t.resetList()
	t.showCurrent()
}

// resetList empties the parameter list.
func (t *GraphingTab) resetList() {
	t.checks = nil
	t.checkIDs = nil
	t.checkList.Objects = nil
	t.checkList.Refresh()
}

// updateChart charts the checked parameters, whether from the open file or
// live.
func (t *GraphingTab) updateChart() {
	switch {
	case t.fileMode:
		t.updateFileChart()
	case t.liveMode():
		t.updateLiveChart()
	default:
		t.updateView()
	}
}

// updateFileChart charts the checked columns of the open file, in column
// order.
func (t *GraphingTab) updateFileChart() {
	if t.log == nil {
		t.charted = false
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

	t.charted = false
	switch {
	case len(t.log.Times) == 0:
		t.status.SetText("This log has no rows")
	case len(columns) == 0:
		t.status.SetText("Choose parameters to graph")
	default:
		t.status.SetText("")
		t.charted = true
	}
	t.updateView()
}

// updateView shows the chart, or the status that explains why there isn't
// one, and the parameter list when there's a log to choose from.
func (t *GraphingTab) updateView() {
	hasList := (t.fileMode && t.log != nil) || (t.liveMode() && len(t.checks) > 0)
	setShown(t.chartView, t.charted)
	setShown(t.statusView, !t.charted)
	setShown(t.chooseParamsBtn, !t.charted && t.needsParams)
	setShown(t.paramsBtn, hasList)
	setShown(t.panel, hasList && t.panelOpen)
	setShown(t.liveBtn, t.fileMode)
	setShown(t.navButtons, t.charted)
	if t.liveMode() {
		if t.paused {
			t.fileName.SetText("Live (paused)")
			t.fitBtn.SetIcon(theme.MediaPlayIcon())
		} else {
			t.fileName.SetText("Live")
			t.fitBtn.SetIcon(theme.ZoomFitIcon())
		}
	} else {
		t.fitBtn.SetIcon(theme.ZoomFitIcon())
	}
	if t.app.connection != nil {
		t.liveBtn.SetText("Live")
		t.liveBtn.SetIcon(theme.MediaPlayIcon())
	} else {
		t.liveBtn.SetText("Close")
		t.liveBtn.SetIcon(theme.CancelIcon())
	}
	if t.view != nil {
		t.buttons.Refresh()
		t.header.Refresh()
		t.view.Refresh()
	}
}

// buttonZoom is how much the zoom buttons scale the time shown.
const buttonZoom = 2.0

// chartBounds is the span of time the chart can be zoomed out to and panned
// within: the whole log, or the live view as of its last refresh.
func (t *GraphingTab) chartBounds() (from, to time.Time) {
	if t.fileMode {
		if t.log == nil || len(t.log.Times) == 0 {
			return from, to
		}
		from, to = t.log.Times[0], t.log.Times[len(t.log.Times)-1]
		if !to.After(from) {
			to = from.Add(time.Second)
		}
		return from, to
	}
	if t.viewNow.IsZero() {
		return from, to
	}
	return t.liveView(t.viewNow)
}

// zoomButton zooms by factor for the zoom buttons: about now on a live chart,
// so the present stays in view, and about the middle otherwise.
func (t *GraphingTab) zoomButton(factor float64) {
	if t.liveMode() && !t.viewNow.IsZero() {
		t.chart.zoomAbout(t.viewNow, factor)
		return
	}
	t.chart.ZoomBy(factor)
}

// fitView zooms the chart out to the whole log, or back to following the
// present when live.
func (t *GraphingTab) fitView() {
	if t.liveMode() {
		t.resumeLive()
		return
	}
	t.chart.Fit()
}

func setShown(o fyne.CanvasObject, shown bool) {
	if shown {
		o.Show()
	} else {
		o.Hide()
	}
}

func (t *GraphingTab) setHint(s string) {
	t.charted, t.needsParams = false, false
	t.status.SetText(s)
	t.updateView()
}

func (t *GraphingTab) setError(s string) {
	t.charted, t.needsParams = false, false
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
