package main

import (
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

type ParametersTab struct {
	app *App

	layout    fyne.Layout
	container *fyne.Container

	// inputs are the parameters' checkboxes and unit selects
	inputs []fyne.Disableable
}

func NewParametersTab(app *App) *ParametersTab {
	paramsLayout := layout.NewGridLayoutWithColumns(4)
	return &ParametersTab{
		app:       app,
		layout:    paramsLayout,
		container: container.New(paramsLayout),
	}
}

func (t *ParametersTab) Container() fyne.CanvasObject {
	return container.NewVScroll(t.container)
}

func (t *ParametersTab) setAvailableParameters(ecu *ssm2.ECU) {
	if ecu == nil {
		ecu = &ssm2.ECU{}
	}
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

	t.container.RemoveAll()
	t.inputs = nil

	loggedParams := t.app.loggedParams.Snapshot()
	for _, param := range params {
		param := param

		options := unitOptions(param.Unit)
		unit := widget.NewSelect(options, func(s string) {
			t.app.loggedParams.SetUnit(param.Id, units.Unit(s))
			// rebuild the live log so its min/max don't mix units
			if lp, ok := t.app.loggedParams.Get(param.Id); ok && lp.LiveLog {
				t.app.LoggingTab.updateLiveLogParameters()
			}
		})
		lp, logged := loggedParams[param.Id]
		if logged {
			unit.Selected = string(lp.Unit)
		} else {
			unit.Selected = options[0]
		}

		fileLogCheck := widget.NewCheck("Log To File", func(b bool) {
			t.app.loggedParams.SetLogToFile(param.Id, param.Derived, units.Unit(unit.Selected), b)
			t.app.LoggingTab.onLoggedParametersChanged()
			t.app.LoggingTab.updateLiveLogParameters()
		})
		liveLogCheck := widget.NewCheck("Live Log", func(b bool) {
			t.app.loggedParams.SetLiveLog(param.Id, param.Derived, units.Unit(unit.Selected), b)
			t.app.LoggingTab.onLoggedParametersChanged()
			t.app.LoggingTab.updateLiveLogParameters()
		})
		fileLogCheck.Checked = lp.LogToFile
		liveLogCheck.Checked = lp.LiveLog
		t.inputs = append(t.inputs, fileLogCheck, liveLogCheck, unit)

		t.container.Objects = append(t.container.Objects,
			NewWrappedLabel(param.Name),
			container.NewCenter(fileLogCheck),
			container.NewCenter(liveLogCheck),
			container.NewCenter(unit),
		)
	}

	t.container.Resize(t.layout.MinSize(t.container.Objects))
	t.container.Refresh()
}

// unitOptions returns the units a parameter can be shown in: its default
// unit, then the units it converts to in a stable order.
func unitOptions(defaultUnit units.Unit) []string {
	options := []string{string(defaultUnit)}
	for _, u := range units.ConvertibleTo(defaultUnit) {
		options = append(options, string(u))
	}
	return options
}

func (t *ParametersTab) toggleParameterChanges(enable bool) {
	for _, input := range t.inputs {
		if enable {
			input.Enable()
		} else {
			input.Disable()
		}
	}
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
