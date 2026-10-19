package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

type LoggingTab struct {
	app *App

	toolbar  *widget.Toolbar
	startBtn *widget.ToolbarAction
	stopBtn  *widget.ToolbarAction

	container *fyne.Container

	runner *logging.Runner
	// paused keeps the logging session stopped while the connection is
	// used for something else.
	paused bool

	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex

	logFile io.WriteCloser
}

func NewLoggingTab(app *App) *LoggingTab {
	loggingTab := &LoggingTab{
		app:       app,
		toolbar:   widget.NewToolbar(),
		startBtn:  widget.NewToolbarAction(theme.MediaPlayIcon(), nil),
		stopBtn:   widget.NewToolbarAction(theme.MediaStopIcon(), nil),
		container: container.New(layout.NewGridLayout(3)),
		runner:    logging.NewRunner(app.loggedParams.Unit, logger),
	}
	loggingTab.startBtn.OnActivated = loggingTab.startFileLogging
	loggingTab.stopBtn.OnActivated = loggingTab.stopFileLogging
	loggingTab.runner.Subscribe("liveLogModels", loggingTab.updateLiveLogModelValues)
	loggingTab.onLoggedParametersChanged()

	return loggingTab
}

func (t *LoggingTab) Container() fyne.CanvasObject {
	return container.NewBorder(t.toolbar, nil, nil, nil, container.NewVScroll(t.container))
}

// EnableLogging starts a logging session for the logged parameters. When
// there is nothing to be logged, any existing session is stopped.
func (t *LoggingTab) EnableLogging() {
	if t.paused || t.app.connection == nil || t.app.loggedParams.Len() == 0 {
		t.runner.Stop()
		return
	}
	params, derived := t.app.loggedParams.Lists()
	t.runner.Start(t.app.connection, params, derived)
}

// PauseLogging stops the logging session until ResumeLogging is called.
// Unlike DisableLogging, file logging stays on and continues on resume.
func (t *LoggingTab) PauseLogging() {
	t.paused = true
	t.runner.Stop()
}

// ResumeLogging restarts the logging session stopped by PauseLogging.
func (t *LoggingTab) ResumeLogging() {
	t.paused = false
	t.EnableLogging()
}

func (t *LoggingTab) DisableLogging() {
	t.runner.Stop()

	if t.logFile != nil {
		t.stopFileLogging()
	}
}

func (t *LoggingTab) startFileLogging() {
	// create the log directory if it doesn't already exist
	logDir := t.app.config.LogDirectory
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		logger.Debugf("creating directory for file logging: %v\n", err)
		return
	}

	// open the log file
	logFileName := logging.FileName(t.app.config.LogFileNameFormat, t.app.ecu.ROM_ID, time.Now())
	f, err := os.OpenFile(filepath.Join(logDir, logFileName), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		logger.Debugf("opening file for logging: %v\n", err)
		return
	}

	// write the file header with only the parameters logged to file;
	// the session also reads the live logged ones
	params, derived := t.app.loggedParams.FileLists()
	csvWriter, err := logging.NewCSVWriter(f, params, derived, t.unitFor)
	if err != nil {
		logger.Debugf("writing log file header: %v\n", err)
		f.Close()
		return
	}
	t.logFile = f

	// don't allow parameter changes while logging to file
	// to keep file results consistent
	t.app.ParametersTab.toggleParameterChanges(false)

	// remove the start button from the toolbar and add the stop button
	t.toolbar.Items = []widget.ToolbarItem{}
	t.toolbar.Append(t.stopBtn)

	t.runner.Subscribe("fileLogging", func(values map[string]ssm2.ParameterValue) {
		if err := csvWriter.Write(time.Now(), values); err != nil {
			logger.Debugf("writing log file row: %v\n", err)
		}
	})
}

func (t *LoggingTab) stopFileLogging() {
	// stop the file logging
	t.runner.Unsubscribe("fileLogging")
	t.logFile.Close()
	t.logFile = nil

	// re-enable all the parameter input
	t.app.ParametersTab.toggleParameterChanges(true)

	// remove the stop button from the toolbar and add the start button
	t.toolbar.Items = []widget.ToolbarItem{}
	t.toolbar.Append(t.startBtn)
}

// unitFor returns the configured unit for a logged parameter.
func (t *LoggingTab) unitFor(id string, defaultUnit units.Unit) units.Unit {
	if lp, ok := t.app.loggedParams.Get(id); ok {
		return lp.Unit
	}
	return defaultUnit
}

type liveLogModel struct {
	Id                  string
	Name                string
	CurrentValueBinding binding.String
	UnitBinding         binding.String

	MaxValueBinding binding.String
	MinValueBinding binding.String
}

func newLiveLogModel(id, name string) *liveLogModel {
	m := &liveLogModel{
		Id:                  id,
		Name:                name,
		CurrentValueBinding: binding.NewString(),
		UnitBinding:         binding.NewString(),
		MaxValueBinding:     binding.NewString(),
		MinValueBinding:     binding.NewString(),
	}
	m.CurrentValueBinding.Set("0")
	m.MaxValueBinding.Set("0")
	m.MinValueBinding.Set("0")
	return m
}

func (m *liveLogModel) Update(val ssm2.ParameterValue, stats logging.Stats) {
	m.CurrentValueBinding.Set(formatLiveValue(val.Value))
	m.UnitBinding.Set(string(val.Unit))
	m.MinValueBinding.Set(formatLiveValue(stats.Min))
	m.MaxValueBinding.Set(formatLiveValue(stats.Max))
}

func formatLiveValue(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', 2, 32)
}

func (t *LoggingTab) updateLiveLogParameters() {
	t.container.RemoveAll()
	t.DisableLogging()

	t.liveLogModelsMu.Lock()
	t.liveLogModels = []*liveLogModel{}
	if t.app.connection == nil {
		t.liveLogModelsMu.Unlock()
		t.container.Refresh()
		return
	}

	loggedParams := t.app.loggedParams.Snapshot()
	for id, param := range loggedParams {
		if !param.LiveLog {
			continue
		}

		var name string
		if param.Derived {
			name = ssm2.DerivedParameters[id].Name
		} else {
			name = ssm2.Parameters[id].Name
		}

		t.liveLogModels = append(t.liveLogModels, newLiveLogModel(id, name))
	}
	sort.Sort(sortableLiveLogModels(t.liveLogModels))
	t.liveLogModelsMu.Unlock()

	for _, m := range t.liveLogModels {
		label := widget.NewLabel(m.Name)
		label.Wrapping = fyne.TextWrapWord
		t.container.Objects = append(t.container.Objects,
			container.NewVBox(
				label,
				container.NewHBox(
					widget.NewLabelWithData(m.CurrentValueBinding),
					widget.NewLabelWithData(m.UnitBinding),
				),
				container.NewHBox(
					widget.NewLabelWithData(m.MinValueBinding),
					widget.NewLabel("/"),
					widget.NewLabelWithData(m.MaxValueBinding),
				),
			))
	}

	t.container.Refresh()

	// file logging also needs a session, even if nothing is live logged
	t.EnableLogging()
}

func (t *LoggingTab) onLoggedParametersChanged() {
	params, derived := t.app.loggedParams.FileLists()
	if len(params)+len(derived) > 0 {
		if len(t.toolbar.Items) == 0 {
			t.toolbar.Append(t.startBtn)
		}
	} else {
		if len(t.toolbar.Items) > 0 {
			t.toolbar.Items = []widget.ToolbarItem{}
		}
	}
}

func (t *LoggingTab) updateLiveLogModelValues(values map[string]ssm2.ParameterValue) {
	t.liveLogModelsMu.Lock()
	for _, m := range t.liveLogModels {
		v, ok := values[m.Id]
		if !ok {
			continue
		}
		stats, _ := t.runner.Stats(m.Id)
		m.Update(v, stats)
	}
	t.liveLogModelsMu.Unlock()
}

type sortableLiveLogModels []*liveLogModel

func (a sortableLiveLogModels) Len() int           { return len(a) }
func (a sortableLiveLogModels) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableLiveLogModels) Less(i, j int) bool { return a[i].Name < a[j].Name }
