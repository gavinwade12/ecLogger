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

	for _, param := range params {
		param := param

		name := widget.NewLabel(param.Name)
		name.Wrapping = fyne.TextWrapBreak

		logCheck := widget.NewCheck("Log To File", func(b bool) {
			if b {
				if config.LoggedParams[param.Id] != nil {
					config.LoggedParams[param.Id].LogToFile = true
				} else {
					config.LoggedParams[param.Id] = &loggedParam{Derived: param.Derived, LogToFile: true}
				}
			} else {
				lp := config.LoggedParams[param.Id]
				if lp != nil && lp.LiveLog {
					lp.LogToFile = false
				} else {
					delete(config.LoggedParams, param.Id)
				}
			}
		})
		logCheck.SetChecked(config.LoggedParams[param.Id] != nil)
		liveLogCheck := widget.NewCheck("Live Log", func(b bool) {
			if b {
				if config.LoggedParams[param.Id] != nil {
					config.LoggedParams[param.Id].LiveLog = true
				} else {
					config.LoggedParams[param.Id] = &loggedParam{Derived: param.Derived, LiveLog: true}
				}
			} else {
				lp := config.LoggedParams[param.Id]
				if lp != nil && lp.LogToFile {
					lp.LiveLog = false
				} else {
					delete(config.LoggedParams, param.Id)
				}
			}
			updateLiveLogParameters()
		})
		liveLogCheck.SetChecked(config.LoggedParams[param.Id] != nil && config.LoggedParams[param.Id].LiveLog)

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

type sortableParameters []parameterModel

func (a sortableParameters) Len() int           { return len(a) }
func (a sortableParameters) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableParameters) Less(i, j int) bool { return a[i].Name < a[j].Name }
