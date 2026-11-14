package main

import (
	"fmt"
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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

type LoggingTab struct {
	app *App

	// header holds the file logging controls, and is hidden when nothing
	// is logged to file
	header     *fyne.Container
	startBtn   *widget.Button
	stopBtn    *widget.Button
	fileStatus *widget.Label

	container *fyne.Container
	// empty explains how to live log parameters when none are
	empty           fyne.CanvasObject
	chooseParamsBtn *widget.Button

	runner *logging.Runner
	// paused keeps the logging session stopped while the connection is
	// used for something else.
	paused bool

	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex

	logFile io.WriteCloser
	// logPath is the path of logFile, while it's open
	logPath string
}

func NewLoggingTab(app *App) *LoggingTab {
	loggingTab := &LoggingTab{
		app:        app,
		startBtn:   widget.NewButtonWithIcon("Log To File", theme.NewErrorThemedResource(theme.MediaRecordIcon()), nil),
		stopBtn:    widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), nil),
		fileStatus: widget.NewLabel(""),
		container:  container.New(newTileGridLayout()),
		runner:     logging.NewRunner(app.loggedParams.Unit, logger),
	}
	loggingTab.stopBtn.Importance = widget.DangerImportance
	loggingTab.stopBtn.Hide()
	loggingTab.fileStatus.Truncation = fyne.TextTruncateEllipsis
	loggingTab.chooseParamsBtn = widget.NewButton("Choose Parameters", func() {
		app.SelectTab(TabParameters)
	})
	loggingTab.empty = container.NewCenter(container.NewVBox(
		newHint("No parameters are live logged"),
		container.NewCenter(loggingTab.chooseParamsBtn),
	))
	loggingTab.header = container.NewPadded(container.NewBorder(nil, nil,
		container.NewHBox(loggingTab.startBtn, loggingTab.stopBtn), nil,
		loggingTab.fileStatus))
	loggingTab.startBtn.OnTapped = loggingTab.startFileLogging
	loggingTab.stopBtn.OnTapped = loggingTab.stopFileLogging
	loggingTab.runner.Subscribe("liveLogModels", loggingTab.updateLiveLogModelValues)
	loggingTab.onLoggedParametersChanged()

	return loggingTab
}

func (t *LoggingTab) Container() fyne.CanvasObject {
	return container.NewBorder(t.header, nil, nil, nil, container.NewStack(
		container.NewVScroll(container.NewPadded(t.container)),
		t.empty,
	))
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
	f, csvWriter, err := t.openLogFile()
	if err != nil {
		logger.Debug(err.Error())
		t.fileStatus.Importance = widget.DangerImportance
		t.fileStatus.SetText("Couldn't log to file: " + err.Error())
		return
	}
	t.logFile = f
	t.logPath = f.Name()

	// don't allow parameter changes while logging to file
	// to keep file results consistent
	t.app.ParametersTab.toggleParameterChanges(false)

	t.startBtn.Hide()
	t.stopBtn.Show()
	t.fileStatus.Importance = widget.MediumImportance
	t.fileStatus.SetText("Logging to " + filepath.Base(f.Name()))
	t.app.LogFilesTab.refresh()

	t.runner.Subscribe("fileLogging", func(values map[string]ssm2.ParameterValue) {
		if err := csvWriter.Write(time.Now(), values); err != nil {
			logger.Debugf("writing log file row: %v\n", err)
		}
	})
}

// openLogFile creates a log file and writes its header.
func (t *LoggingTab) openLogFile() (*os.File, *logging.CSVWriter, error) {
	// create the log directory if it doesn't already exist
	logDir := t.app.config.LogDirectory
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating the log directory: %w", err)
	}

	logFileName := logging.FileName(t.app.config.LogFileNameFormat, t.app.ecu.ROM_ID, time.Now())
	f, err := os.OpenFile(filepath.Join(logDir, logFileName), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("opening the log file: %w", err)
	}

	// write the file header with only the parameters logged to file;
	// the session also reads the live logged ones
	params, derived := t.app.loggedParams.FileLists()
	csvWriter, err := logging.NewCSVWriter(f, params, derived, t.unitFor)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("writing the log file header: %w", err)
	}
	return f, csvWriter, nil
}

func (t *LoggingTab) stopFileLogging() {
	// stop the file logging
	t.runner.Unsubscribe("fileLogging")
	t.logFile.Close()
	t.logFile = nil
	t.logPath = ""

	// re-enable all the parameter input
	t.app.ParametersTab.toggleParameterChanges(true)

	t.stopBtn.Hide()
	t.startBtn.Show()
	t.fileStatus.SetText("")
	t.app.LogFilesTab.refresh()
}

// CurrentLogFile returns the path of the file being logged to, or "" when
// not logging to a file.
func (t *LoggingTab) CurrentLogFile() string { return t.logPath }

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

// noValue is shown for a live logged parameter until it has a value.
const noValue = "–"

func newLiveLogModel(id, name string) *liveLogModel {
	m := &liveLogModel{
		Id:                  id,
		Name:                name,
		CurrentValueBinding: binding.NewString(),
		UnitBinding:         binding.NewString(),
		MaxValueBinding:     binding.NewString(),
		MinValueBinding:     binding.NewString(),
	}
	// a placeholder rather than 0, which looks like a real reading
	m.CurrentValueBinding.Set(noValue)
	m.MaxValueBinding.Set(noValue)
	m.MinValueBinding.Set(noValue)
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
		t.empty.Show()
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
		t.container.Objects = append(t.container.Objects, newLiveLogTile(m))
	}
	if len(t.liveLogModels) == 0 {
		t.empty.Show()
	} else {
		t.empty.Hide()
	}

	t.container.Refresh()

	// file logging also needs a session, even if nothing is live logged
	t.EnableLogging()
}

func (t *LoggingTab) onLoggedParametersChanged() {
	params, derived := t.app.loggedParams.FileLists()
	if len(params)+len(derived) > 0 {
		t.header.Show()
	} else {
		t.header.Hide()
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
