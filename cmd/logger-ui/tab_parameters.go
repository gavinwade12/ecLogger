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

	loggedParams := t.app.loggedParams.CopyData()
	for _, param := range params {
		param := param

		options := make([]string, 1+len(units.UnitConversions[param.Unit]))
		options[0] = string(param.Unit)
		i := 1
		for u := range units.UnitConversions[param.Unit] {
			options[i] = string(u)
			i++
		}

		unit := widget.NewSelect(options, func(s string) {
			lp := t.app.loggedParams.Get(param.Id)
			if lp != nil {
				lp.Unit = units.Unit(s)
			}
		})
		lp := loggedParams[param.Id]
		if lp != nil {
			unit.Selected = string(lp.Unit)
		} else {
			unit.Selected = options[0]
		}

		fileLogCheck := widget.NewCheck("Log To File", func(b bool) {
			if b {
				t.app.loggedParams.UpdateOrAdd(param.Id, func(lp *LoggedParam) {
					lp.LogToFile = true
				}, &LoggedParam{Derived: param.Derived, LogToFile: true, Unit: units.Unit(unit.Selected)})
			} else {
				lp := t.app.loggedParams.Get(param.Id)
				if lp != nil && lp.LiveLog {
					t.app.loggedParams.Update(param.Id, func(lp *LoggedParam) {
						lp.LogToFile = false
					})
				} else {
					t.app.loggedParams.Remove(param.Id)
				}
			}
			t.app.LoggingTab.onLoggedParametersChanged()
		})
		liveLogCheck := widget.NewCheck("Live Log", func(b bool) {
			if b {
				t.app.loggedParams.UpdateOrAdd(param.Id, func(lp *LoggedParam) {
					lp.LiveLog = true
				}, &LoggedParam{Derived: param.Derived, LiveLog: true, Unit: units.Unit(unit.Selected)})
			} else {
				lp := t.app.loggedParams.Get(param.Id)
				if lp != nil && lp.LogToFile {
					t.app.loggedParams.Update(param.Id, func(lp *LoggedParam) {
						lp.LiveLog = false
					})
				} else {
					t.app.loggedParams.Remove(param.Id)
				}
			}
			t.app.LoggingTab.onLoggedParametersChanged()
			t.app.LoggingTab.updateLiveLogParameters()
		})
		fileLogCheck.Checked = loggedParams[param.Id] != nil && loggedParams[param.Id].LogToFile
		liveLogCheck.Checked = loggedParams[param.Id] != nil && loggedParams[param.Id].LiveLog

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

func (t *ParametersTab) toggleParameterChanges(enable bool) {
	for i, o := range t.container.Objects {
		if i%4 == 0 {
			continue // skip the first column since it's just text
		}

		traverseObjectAndToggle(enable, o)
	}
}

func traverseObjectAndToggle(enable bool, o fyne.CanvasObject) {
	switch w := o.(type) {
	case *fyne.Container:
		for _, oo := range w.Objects {
			traverseObjectAndToggle(enable, oo)
		}
	case fyne.Disableable:
		toggleEnable(enable, w)
	}
}

func toggleEnable(enable bool, o fyne.Disableable) {
	if enable {
		o.Enable()
	} else {
		o.Disable()
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
