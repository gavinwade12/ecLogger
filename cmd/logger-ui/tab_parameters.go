package main

import (
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

type ParametersTab struct {
	app *App

	// header names the columns of rows, and stays put as rows scroll
	header *fyne.Container
	rows   *fyne.Container
	// locked explains why the inputs are disabled while logging to file
	locked *hint

	// inputs are the parameters' checkboxes and unit selects
	inputs []fyne.Disableable
}

func NewParametersTab(app *App) *ParametersTab {
	t := &ParametersTab{
		app:    app,
		header: container.NewPadded(),
		rows:   container.New(newListLayout(0)),
		locked: newHint("Parameters can't be changed while logging to file"),
	}
	t.locked.Hide()
	return t
}

func (t *ParametersTab) Container() fyne.CanvasObject {
	return container.NewBorder(
		container.NewVBox(t.locked, t.header), nil, nil, nil,
		container.NewVScroll(container.NewPadded(t.rows)),
	)
}

// The column headings, and the column the parameter names are in.
const (
	headingName      = "Parameter"
	headingLogToFile = "Log To File"
	headingLiveLog   = "Live Log"
	headingUnit      = "Unit"
	nameColumn       = 0
)

// columnWidths returns the widths of the Log To File, Live Log and unit
// columns, so their headings and inputs fit. options are the unit
// options of every parameter.
func columnWidths(options [][]string) []float32 {
	textSize := theme.TextSize()
	headingWidth := func(heading string) float32 {
		return fyne.MeasureText(heading, textSize, fyne.TextStyle{}).Width
	}
	checkWidth := widget.NewCheck("", nil).MinSize().Width

	// a select is as wide as its text, its padding and its icon
	unitWidth := headingWidth(headingUnit)
	selectChrome := widget.NewSelect(nil, nil).MinSize().Width -
		headingWidth(widget.NewSelect(nil, nil).PlaceHolder)
	for _, opts := range options {
		for _, o := range opts {
			unitWidth = max(unitWidth, headingWidth(o)+selectChrome)
		}
	}

	return []float32{
		max(headingWidth(headingLogToFile), checkWidth),
		max(headingWidth(headingLiveLog), checkWidth),
		unitWidth,
	}
}

func newColumnHeading(text string, alignment fyne.TextAlign) *wrappedText {
	heading := newWrappedText(text, theme.SizeNameText, colorNameSecondaryText)
	heading.alignment = alignment
	return heading
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

	t.rows.RemoveAll()
	t.inputs = nil

	options := make([][]string, len(params))
	for i, param := range params {
		options[i] = unitOptions(param.Unit)
	}
	widths := columnWidths(options)
	t.header.Objects = []fyne.CanvasObject{newColumnsRow(widths, nameColumn, "",
		newColumnHeading(headingName, fyne.TextAlignLeading),
		newColumnHeading(headingLogToFile, fyne.TextAlignCenter),
		newColumnHeading(headingLiveLog, fyne.TextAlignCenter),
		newColumnHeading(headingUnit, fyne.TextAlignCenter),
	)}
	t.header.Refresh()

	loggedParams := t.app.loggedParams.Snapshot()
	for i, param := range params {
		param := param

		options := options[i]
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

		fileLogCheck := widget.NewCheck("", func(b bool) {
			t.app.loggedParams.SetLogToFile(param.Id, param.Derived, units.Unit(unit.Selected), b)
			t.app.LoggingTab.onLoggedParametersChanged()
			t.app.LoggingTab.updateLiveLogParameters()
		})
		liveLogCheck := widget.NewCheck("", func(b bool) {
			t.app.loggedParams.SetLiveLog(param.Id, param.Derived, units.Unit(unit.Selected), b)
			t.app.LoggingTab.onLoggedParametersChanged()
			t.app.LoggingTab.updateLiveLogParameters()
		})
		fileLogCheck.Checked = lp.LogToFile
		liveLogCheck.Checked = lp.LiveLog
		t.inputs = append(t.inputs, fileLogCheck, liveLogCheck, unit)

		// every other row is shaded, to follow a row across
		var background fyne.ThemeColorName
		if i%2 == 1 {
			background = colorNameTile
		}
		t.rows.Objects = append(t.rows.Objects, newColumnsRow(widths, nameColumn, background,
			newWrappedText(param.Name, theme.SizeNameText, theme.ColorNameForeground),
			container.NewCenter(fileLogCheck),
			container.NewCenter(liveLogCheck),
			unit,
		))
	}

	t.rows.Refresh()
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
	if enable {
		t.locked.Hide()
	} else {
		t.locked.Show()
	}
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
