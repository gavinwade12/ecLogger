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

var loggingTab *LoggingTab

type LoggingTab struct {
	toolbar  *widget.Toolbar
	startBtn *widget.ToolbarAction
	stopBtn  *widget.ToolbarAction

	container *fyne.Container

	loggingProcessors   map[string]func(map[string]ssm2.ParameterValue)
	loggingProcessorsMu sync.Mutex

	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex

	stopLogging context.CancelFunc
}

func NewLoggingTab() *LoggingTab {
	loggingTab := &LoggingTab{
		toolbar:           widget.NewToolbar(),
		startBtn:          widget.NewToolbarAction(theme.MediaPlayIcon(), nil),
		stopBtn:           widget.NewToolbarAction(theme.MediaStopIcon(), nil),
		container:         container.New(layout.NewGridLayout(3)),
		loggingProcessors: map[string]func(map[string]ssm2.ParameterValue){},
	}

	loggingTab.startBtn.OnActivated = func() {
		// open the log file
		logDir := *config.LogDirectory
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			logger.Debugf("creating directory for file logging: %v\n", err)
			return
		}

		logFileName := strings.NewReplacer(
			"{{romId}}", hex.EncodeToString(ecu.ROM_ID),
			"{{timestamp}}", time.Now().Format("20060102_150405"), //yyyyMMdd_hhmmss
		).Replace(*config.LogFileNameFormat)

		var err error
		loggingFile, err = os.OpenFile(
			path.Join(logDir, logFileName),
			os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			logger.Debugf("opening file for logging: %v\n", err)
			return
		}

		// don't allow parameter changes while logging to file
		// to keep file results consistent
		for i, o := range paramsContainer.Objects {
			if i%4 == 0 {
				continue // skip the first column since it's just text
			}
			switch w := o.(type) {
			case *widget.Check:
				w.Disable()
			case *widget.Select:
				w.Disable()
			}
		}

		// write the file header
		loggingFile.Write([]byte("Timestamp,"))
		params, derived := getCurrentLoggedParamLists()
		loggedParams := readOnlyLoggedParams()
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
			loggingFile.Write([]byte(val))
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
			loggingFile.Write([]byte(val))
		}

		// remove this button from the toolbar and add the stop button
		loggingTab.toolbar.Items = []widget.ToolbarItem{}
		loggingTab.toolbar.Append(loggingTab.stopBtn)

		loggingTab.setLoggingProcessor("fileLogging", updateFileLogValues(params, derived))
	}
	loggingTab.stopBtn.OnActivated = func() {
		loggingTab.removeLoggingProcessor("fileLogging")
		loggingFile.Close()
		loggingFile = nil

		// re-enable all the parameter input
		for i, o := range paramsContainer.Objects {
			if i%4 == 0 {
				continue // skip the first column since it's just text
			}
			switch w := o.(type) {
			case *widget.Check:
				w.Enable()
			case *widget.Select:
				w.Enable()
			}
		}

		// remove this button from the toolbar and add the start button
		loggingTab.toolbar.Items = []widget.ToolbarItem{}
		loggingTab.toolbar.Append(loggingTab.startBtn)
	}

	loggingTab.loggingProcessors["liveLogModels"] = loggingTab.updateLiveLogModelValues
	loggingTab.onLoggedParametersChanged()

	return loggingTab
}

func (t *LoggingTab) Container() fyne.CanvasObject {
	return container.NewBorder(t.toolbar, nil, nil, nil, container.NewVScroll(t.container))
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
	loggingTab.container.RemoveAll()

	if t.stopLogging != nil {
		t.stopLogging()
		t.stopLogging = nil
	}

	t.liveLogModelsMu.Lock()
	t.liveLogModels = []*liveLogModel{}
	if conn == nil {
		t.liveLogModelsMu.Unlock()
		loggingTab.container.Refresh()
		return
	}

	loggedParams := readOnlyLoggedParams()
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
		var ctx context.Context
		ctx, t.stopLogging = context.WithCancel(context.Background())

		go t.startLogging(ctx)
	}
}

func (t *LoggingTab) startLogging(ctx context.Context) {
	var (
		session               <-chan map[string]ssm2.ParameterValue
		err                   error
		params, derivedParams = getCurrentLoggedParamLists()
	)
	for {
		session, err = ssm2.LoggingSession(ctx, conn, params, derivedParams)
		if err == nil {
			break
		}

		logger.Debug(err.Error())
	}

	for {
		select {
		case <-ctx.Done():
			return
		case result := <-session:
			if result == nil {
				t.updateLiveLogParameters()
				return
			}

			// convert the result values to the configured units
			loggedParams := readOnlyLoggedParams()
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
	}
}

func (t *LoggingTab) onLoggedParametersChanged() {
	loggedParams := readOnlyLoggedParams()
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

func getCurrentLoggedParamLists() ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	params := []ssm2.Parameter{}
	derivedParams := []ssm2.DerivedParameter{}
	loggedParams := readOnlyLoggedParams()
	for id, p := range loggedParams {
		if p.Derived {
			derivedParams = append(derivedParams, ssm2.DerivedParameters[id])
		} else {
			params = append(params, ssm2.Parameters[id])
		}
	}

	return params, derivedParams
}

var loggingFile io.WriteCloser

func updateFileLogValues(params []ssm2.Parameter, derived []ssm2.DerivedParameter) func(values map[string]ssm2.ParameterValue) {
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
		loggingFile.Write([]byte(time.Now().Format("2006-01-02 15:04:05.999999999") + ",")) // yyyy-MM-dd hh:mm:ss
		for i, id := range order {
			val := strconv.FormatFloat(float64(values[id].Value), 'f', 4, 32)
			if i < len(order)-1 {
				val += ","
			} else {
				val += "\n"
			}
			loggingFile.Write([]byte(val))
		}
	}
}

type sortableLiveLogModels []*liveLogModel

func (a sortableLiveLogModels) Len() int           { return len(a) }
func (a sortableLiveLogModels) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableLiveLogModels) Less(i, j int) bool { return a[i].Name < a[j].Name }
