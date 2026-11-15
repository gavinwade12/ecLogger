package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

type SettingsTab struct {
	form *widget.Form
}

// a package-level variable so extras can be added in a dev build
var settingsFormItems = []func(app *App) []*widget.FormItem{
	func(app *App) []*widget.FormItem {
		return []*widget.FormItem{
			widget.NewFormItem("Log Directory", widget.NewEntryWithData(
				binding.BindString(&app.config.LogDirectory))),
			newFormItemWithHint("Log File Name Format", widget.NewEntryWithData(
				binding.BindString(&app.config.LogFileNameFormat)),
				"{{romId}} and {{timestamp}} are filled in for each log"),
			newFormItemWithHint("Auto Connect", widget.NewCheckWithData("",
				binding.BindBool(&app.config.AutoConnect)),
				"Connect to the selected port when the app starts"),
			newFormItemWithHint("Default to Logging Tab", widget.NewCheckWithData(
				"", binding.BindBool(&app.config.DefaultToLoggingTab)),
				"Open on the Logging tab when the app starts"),
			newFormItemWithHint("Live Graph Window", newLiveGraphWindowSelect(app),
				"How much recent data the Graphing tab shows while connected"),
		}
	},
}

func NewSettingsTab(app *App) *SettingsTab {
	formItems := []*widget.FormItem{}
	for _, f := range settingsFormItems {
		formItems = append(formItems, f(app)...)
	}
	return &SettingsTab{
		form: widget.NewForm(formItems...),
	}
}

func (t *SettingsTab) Container() fyne.CanvasObject {
	return container.NewPadded(newMaxWidth(container.NewVBox(t.form)))
}

// newFormItemWithHint returns a form item with hint shown under it.
func newFormItemWithHint(text string, w fyne.CanvasObject, hint string) *widget.FormItem {
	item := widget.NewFormItem(text, w)
	item.HintText = hint
	return item
}

// liveGraphMinuteChoices are the history lengths offered for the live graph.
var liveGraphMinuteChoices = []int{1, 2, 5, 10, 15, 30}

func minutesLabel(n int) string {
	if n == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", n)
}

// newLiveGraphWindowSelect returns a select of history lengths.
func newLiveGraphWindowSelect(app *App) *widget.Select {
	choices := append([]int(nil), liveGraphMinuteChoices...)
	// a hand-edited config can hold a length that isn't offered
	current := app.config.LiveGraphMinutes
	if current > 0 && !slices.Contains(choices, current) {
		choices = append(choices, current)
		slices.Sort(choices)
	}
	options := make([]string, len(choices))
	for i, n := range choices {
		options[i] = minutesLabel(n)
	}
	sel := widget.NewSelect(options, nil)
	sel.Selected = minutesLabel(current)
	sel.OnChanged = func(s string) {
		n, err := strconv.Atoi(strings.Fields(s)[0])
		if err != nil {
			return
		}
		app.config.LiveGraphMinutes = n
		app.GraphingTab.SetLiveWindow(app.config.LiveGraphWindow())
	}
	return sel
}
