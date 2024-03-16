package main

import (
	"fyne.io/fyne/v2"
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
				binding.BindString(app.config.LogDirectory))),
			widget.NewFormItem("Log File Name Format", widget.NewEntryWithData(
				binding.BindString(app.config.LogFileNameFormat))),
			widget.NewFormItem("Auto Connect", widget.NewCheckWithData("",
				binding.BindBool(&app.config.AutoConnect))),
			widget.NewFormItem("Default to Logging Tab", widget.NewCheckWithData(
				"", binding.BindBool(&app.config.DefaultToLoggingTab))),
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
	return t.form
}
