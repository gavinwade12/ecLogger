package main

import (
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
	"github.com/gavinwade12/ssm2/units"
)

var paramsContainer *fyne.Container

func parametersContainer() fyne.CanvasObject {
	paramsContainer = container.New(layout.NewGridLayout(4))
	return container.NewVScroll(paramsContainer)
}

func setAvailableParameters(ecu *ssm2.ECU) {
	params := make([]parameterModel, len(ecu.SupportedParameters)+
		len(ecu.SupportedDerivedParameters))
	i := 0
	for _, p := range ecu.SupportedParameters {
		params[i] = parameterModel{
			Id:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			Unit:        p.DefaultUnit,
			Derived:     false,
		}
		i++
	}
	for _, p := range ecu.SupportedDerivedParameters {
		params[i] = parameterModel{
			Id:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			Unit:        p.DefaultUnit,
			Derived:     true,
		}
		i++
	}
	sort.Sort(sortableParameters(params))

	paramsContainer.RemoveAll()

	loggedParams := readOnlyLoggedParams()
	for _, param := range params {
		param := param

		name := widget.NewLabel(param.Name)
		name.Wrapping = fyne.TextWrapBreak

		logCheck := widget.NewCheck("Log To File", func(b bool) {
			if b {
				updateOrAddToLoggedParams(param.Id, func(lp *loggedParam) {
					lp.LogToFile = true
				}, &loggedParam{Derived: param.Derived, LogToFile: true})
			} else {
				lp := getLoggedParam(param.Id)
				if lp != nil && lp.LiveLog {
					updateLoggedParam(param.Id, func(lp *loggedParam) {
						lp.LogToFile = false
					})
				} else {
					removeFromLoggedParams(param.Id)
				}
			}
		})
		liveLogCheck := widget.NewCheck("Live Log", func(b bool) {
			if b {
				updateOrAddToLoggedParams(param.Id, func(lp *loggedParam) {
					lp.LiveLog = true
				}, &loggedParam{Derived: param.Derived, LiveLog: true})
			} else {
				lp := getLoggedParam(param.Id)
				if lp != nil && lp.LogToFile {
					updateLoggedParam(param.Id, func(lp *loggedParam) {
						lp.LiveLog = false
					})
				} else {
					removeFromLoggedParams(param.Id)
				}
			}
			updateLiveLogParameters()
		})
		logCheck.Checked = loggedParams[param.Id] != nil
		liveLogCheck.Checked = loggedParams[param.Id] != nil && loggedParams[param.Id].LiveLog

		options := make([]string, 1+len(units.UnitConversions[param.Unit]))
		options[0] = string(param.Unit)
		i := 1
		for u := range units.UnitConversions[param.Unit] {
			options[i] = string(u)
			i++
		}

		unit := widget.NewSelect(options, func(s string) {
			lp := config.LoggedParams[param.Id]
			if lp != nil {
				lp.Unit = units.Unit(s)
			}
		})
		lp := config.LoggedParams[param.Id]
		if lp != nil {
			unit.SetSelected(string(lp.Unit))
		} else {
			unit.SetSelected(options[0])
		}

		paramsContainer.Objects = append(paramsContainer.Objects,
			name,
			logCheck,
			liveLogCheck,
			unit,
		)
	}

	paramsContainer.Refresh()
}

type parameterModel struct {
	Id          string
	Name        string
	Description string
	Derived     bool
	Unit        units.Unit
}

func addToLoggedParams(key string, l *loggedParam) {
	loggedParamsMu.Lock()
	defer loggedParamsMu.Unlock()
	config.LoggedParams[key] = l
}

func removeFromLoggedParams(key string) {
	loggedParamsMu.Lock()
	defer loggedParamsMu.Unlock()
	delete(config.LoggedParams, key)
}

func updateLoggedParam(key string, update func(*loggedParam)) {
	loggedParamsMu.Lock()
	defer loggedParamsMu.Unlock()
	update(config.LoggedParams[key])
}

func updateOrAddToLoggedParams(key string, update func(*loggedParam), add *loggedParam) {
	loggedParamsMu.Lock()
	defer loggedParamsMu.Unlock()

	if config.LoggedParams[key] != nil {
		update(config.LoggedParams[key])
	} else {
		config.LoggedParams[key] = add
	}
}

func getLoggedParam(key string) *loggedParam {
	loggedParamsMu.RLock()
	defer loggedParamsMu.RUnlock()
	return config.LoggedParams[key]
}

func readOnlyLoggedParams() map[string]*loggedParam {
	loggedParamsMu.RLock()
	defer loggedParamsMu.RUnlock()

	m := make(map[string]*loggedParam)
	for k, v := range config.LoggedParams {
		m[k] = v
	}
	return m
}

type sortableParameters []parameterModel

func (a sortableParameters) Len() int           { return len(a) }
func (a sortableParameters) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableParameters) Less(i, j int) bool { return a[i].Name < a[j].Name }