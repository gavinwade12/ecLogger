package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

var liveLogContainer *fyne.Container

func loggingContainer() fyne.CanvasObject {
	startBtn := widget.NewToolbarAction(theme.MediaPlayIcon(), nil)
	stopBtn := widget.NewToolbarAction(theme.MediaStopIcon(), nil)
	toolbar := widget.NewToolbar()

	startBtn.OnActivated = func() {
		// open the log file
		logFileFormat := strings.NewReplacer(
			"{{romId}}", hex.EncodeToString(ecu.ROM_ID),
			"{{timestamp}}", time.Now().Format("20060102_150405"), //yyyyMMdd_hhmmss
		).Replace(*config.LogFileNameFormat)

		var err error
		loggingFile, err = os.OpenFile(logFileFormat, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
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
		toolbar.Items = []widget.ToolbarItem{}
		toolbar.Append(stopBtn)

		setLoggingProcessor("fileLogging", updateFileLogValues(params, derived))
	}
	stopBtn.OnActivated = func() {
		removeLoggingProcessor("fileLogging")
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
		toolbar.Items = []widget.ToolbarItem{}
		toolbar.Append(startBtn)
	}
	toolbar.Append(startBtn)

	liveLogContainer = container.New(layout.NewGridLayout(3))
	return container.NewBorder(toolbar, nil, nil, nil, container.NewVScroll(liveLogContainer))
}

var (
	loggingProcessors = map[string]func(map[string]ssm2.ParameterValue){
		"liveLogModels": updateLiveLogModelValues,
	}
	loggingProcessorsMu sync.Mutex
)

func setLoggingProcessor(key string, p func(map[string]ssm2.ParameterValue)) {
	loggingProcessorsMu.Lock()
	defer loggingProcessorsMu.Unlock()
	loggingProcessors[key] = p
}

func removeLoggingProcessor(key string) {
	loggingProcessorsMu.Lock()
	defer loggingProcessorsMu.Unlock()
	delete(loggingProcessors, key)
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

var (
	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex
)

func updateLiveLogParameters() {
	liveLogContainer.RemoveAll()

	if stopLogging != nil {
		stopLogging()
		stopLogging = nil
	}

	liveLogModelsMu.Lock()
	liveLogModels = []*liveLogModel{}
	if conn == nil {
		liveLogModelsMu.Unlock()
		liveLogContainer.Refresh()
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

		liveLogModels = append(liveLogModels, newLiveLogModel(id, name))
	}
	sort.Sort(sortableLiveLogModels(liveLogModels))
	liveLogModelsLen := len(liveLogModels)
	liveLogModelsMu.Unlock()

	for _, m := range liveLogModels {
		label := widget.NewLabel(m.Name)
		label.Wrapping = fyne.TextWrapWord
		liveLogContainer.Objects = append(liveLogContainer.Objects,
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

	liveLogContainer.Refresh()

	if liveLogModelsLen > 0 {
		var ctx context.Context
		ctx, stopLogging = context.WithCancel(context.Background())
		go startLogging(ctx)
	}
}

var stopLogging context.CancelFunc

func startLogging(ctx context.Context) {
	params, derivedParams := getCurrentLoggedParamLists()

	var (
		session <-chan map[string]ssm2.ParameterValue
		err     error
	)
	for {
		session, err = ssm2.LoggingSession(ctx, conn, params, derivedParams)
		if err != nil {
			logger.Debug(err.Error())
		} else {
			break
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case result := <-session:
			if result == nil {
				updateLiveLogParameters()
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

			loggingProcessorsMu.Lock()
			for _, p := range loggingProcessors {
				p(result)
			}
			loggingProcessorsMu.Unlock()
		}
	}
}

func updateLiveLogModelValues(values map[string]ssm2.ParameterValue) {
	liveLogModelsMu.Lock()
	for _, m := range liveLogModels {
		m.Update(values[m.Id])
	}
	liveLogModelsMu.Unlock()
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
