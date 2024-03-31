package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

type LoggingTab struct {
	app *App

	toolbar  *widget.Toolbar
	startBtn *widget.ToolbarAction
	stopBtn  *widget.ToolbarAction

	container *fyne.Container

	loggingProcessors   map[string]func(map[string]ssm2.ParameterValue)
	loggingProcessorsMu sync.Mutex

	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex

	cancelLogging context.CancelFunc
	doneLogging   chan struct{}
	logFile       io.WriteCloser
}

func NewLoggingTab(app *App) *LoggingTab {
	loggingTab := &LoggingTab{
		app:               app,
		toolbar:           widget.NewToolbar(),
		startBtn:          widget.NewToolbarAction(theme.MediaPlayIcon(), nil),
		stopBtn:           widget.NewToolbarAction(theme.MediaStopIcon(), nil),
		container:         container.New(layout.NewGridLayout(3)),
		loggingProcessors: map[string]func(map[string]ssm2.ParameterValue){},
	}
	loggingTab.startBtn.OnActivated = loggingTab.startFileLogging
	loggingTab.stopBtn.OnActivated = loggingTab.stopFileLogging
	loggingTab.loggingProcessors["liveLogModels"] = loggingTab.updateLiveLogModelValues
	loggingTab.onLoggedParametersChanged()

	return loggingTab
}

func (t *LoggingTab) Container() fyne.CanvasObject {
	return container.NewBorder(t.toolbar, nil, nil, nil, container.NewVScroll(t.container))
}

func (t *LoggingTab) EnableLogging() {
	var ctx context.Context
	ctx, t.cancelLogging = context.WithCancel(context.Background())
	t.doneLogging = make(chan struct{})

	go t.openLoggingSession(ctx)
}

func (t *LoggingTab) DisableLogging() {
	if t.cancelLogging != nil {
		t.cancelLogging()
		t.cancelLogging = nil
		<-t.doneLogging
	}

	if t.logFile != nil {
		t.stopFileLogging()
	}
}

func (t *LoggingTab) startFileLogging() {
	// create the log directory if it doesn't already exist
	logDir := *t.app.config.LogDirectory
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		logger.Debugf("creating directory for file logging: %v\n", err)
		return
	}

	// determine the log file name
	logFileName := strings.NewReplacer(
		"{{romId}}", hex.EncodeToString(t.app.ecu.ROM_ID),
		"{{timestamp}}", time.Now().Format("20060102_150405"), //yyyyMMdd_hhmmss
	).Replace(*t.app.config.LogFileNameFormat)

	// open the log file
	var err error
	t.logFile, err = os.OpenFile(
		path.Join(logDir, logFileName),
		os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		logger.Debugf("opening file for logging: %v\n", err)
		return
	}

	// don't allow parameter changes while logging to file
	// to keep file results consistent
	t.app.ParametersTab.toggleParameterChanges(false)

	// write the file header
	params, derived := t.app.loggedParams.CurrentLists()
	t.writeLogFileHeader(params, derived)

	// remove the start button from the toolbar and add the stop button
	t.toolbar.Items = []widget.ToolbarItem{}
	t.toolbar.Append(t.stopBtn)

	t.setLoggingProcessor("fileLogging", t.updateFileLogValues(params, derived))
}

func (t *LoggingTab) stopFileLogging() {
	// stop the file logging
	t.removeLoggingProcessor("fileLogging")
	t.logFile.Close()
	t.logFile = nil

	// re-enable all the parameter input
	t.app.ParametersTab.toggleParameterChanges(true)

	// remove the stop button from the toolbar and add the start button
	t.toolbar.Items = []widget.ToolbarItem{}
	t.toolbar.Append(t.startBtn)
}

func (t *LoggingTab) writeLogFileHeader(params []ssm2.Parameter, derived []ssm2.DerivedParameter) {
	t.logFile.Write([]byte("Timestamp,"))
	loggedParams := t.app.loggedParams.CopyData()
	for i, p := range params {
		u := p.DefaultUnit
		lp := loggedParams[p.Id]
		if lp != nil {
			u = lp.Unit
		}
		val := fmt.Sprintf("%s (%s)", p.Name, u)

		if len(derived) > 0 || i < len(params)-1 {
			val += ","
		} else {
			val += "\n"
		}
		t.logFile.Write([]byte(val))
	}
	for i, p := range derived {
		u := p.DefaultUnit
		lp := loggedParams[p.Id]
		if lp != nil {
			u = lp.Unit
		}
		val := fmt.Sprintf("%s (%s)", p.Name, u)

		if i < len(derived)-1 {
			val += ","
		} else {
			val += "\n"
		}
		t.logFile.Write([]byte(val))
	}
}

func (t *LoggingTab) setLoggingProcessor(key string, p func(map[string]ssm2.ParameterValue)) {
	t.loggingProcessorsMu.Lock()
	defer t.loggingProcessorsMu.Unlock()
	t.loggingProcessors[key] = p
}

func (t *LoggingTab) removeLoggingProcessor(key string) {
	t.loggingProcessorsMu.Lock()
	defer t.loggingProcessorsMu.Unlock()
	delete(t.loggingProcessors, key)
}

type liveLogModel struct {
	Id                  string
	Name                string
	CurrentValueBinding binding.String
	UnitBinding         binding.String

	MaxValue        float32
	MaxValueBinding binding.String
	MinValue        float32
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

func (m *liveLogModel) Update(val ssm2.ParameterValue) {
	f := strconv.FormatFloat(float64(val.Value), 'f', 2, 32)
	m.CurrentValueBinding.Set(f)
	m.UnitBinding.Set(string(val.Unit))

	if val.Value > m.MaxValue {
		m.MaxValue = val.Value
		m.MaxValueBinding.Set(f)
	}
	if val.Value < m.MinValue {
		m.MinValue = val.Value
		m.MinValueBinding.Set(f)
	}
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

	loggedParams := t.app.loggedParams.CopyData()
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
	liveLogModelsLen := len(t.liveLogModels)
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

	if liveLogModelsLen > 0 {
		t.EnableLogging()
	}
}

func (t *LoggingTab) openLoggingSession(ctx context.Context) {
	var (
		session               <-chan map[string]ssm2.ParameterValue
		err                   error
		params, derivedParams = t.app.loggedParams.CurrentLists()
	)
	for {
		session, err = ssm2.LoggingSession(ctx, t.app.connection, params, derivedParams)
		if err == nil {
			break
		}

		logger.Debug(err.Error())
	}

	for result := range session {
		// convert the result values to the configured units
		loggedParams := t.app.loggedParams.CopyData()
		for id, val := range result {
			lp := loggedParams[id]
			if lp == nil || lp.Unit == val.Unit {
				continue
			}

			vval, err := val.ConvertTo(lp.Unit)
			if err != nil {
				logger.Debugf("converting %s from %s to %s: %v\n", id, val.Unit, lp.Unit, err)
				continue
			}
			result[id] = *vval
		}

		t.loggingProcessorsMu.Lock()
		for _, p := range t.loggingProcessors {
			p(result)
		}
		t.loggingProcessorsMu.Unlock()
	}

	t.doneLogging <- struct{}{}
}

func (t *LoggingTab) onLoggedParametersChanged() {
	loggedParams := t.app.loggedParams.CopyData()
	if len(loggedParams) > 0 {
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
		m.Update(values[m.Id])
	}
	t.liveLogModelsMu.Unlock()
}

func (t *LoggingTab) updateFileLogValues(params []ssm2.Parameter, derived []ssm2.DerivedParameter) func(values map[string]ssm2.ParameterValue) {
	order := make([]string, len(params)+len(derived))
	i := 0
	for _, p := range params {
		order[i] = p.Id
		i++
	}
	for _, p := range derived {
		order[i] = p.Id
		i++
	}

	return func(values map[string]ssm2.ParameterValue) {
		t.logFile.Write([]byte(time.Now().Format("2006-01-02 15:04:05.999999999") + ",")) // yyyy-MM-dd hh:mm:ss
		for i, id := range order {
			val := strconv.FormatFloat(float64(values[id].Value), 'f', 4, 32)
			if i < len(order)-1 {
				val += ","
			} else {
				val += "\n"
			}
			t.logFile.Write([]byte(val))
		}
	}
}

type sortableLiveLogModels []*liveLogModel

func (a sortableLiveLogModels) Len() int           { return len(a) }
func (a sortableLiveLogModels) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableLiveLogModels) Less(i, j int) bool { return a[i].Name < a[j].Name }
