package main

import (
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
