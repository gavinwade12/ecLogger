package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func settingsContainer() fyne.CanvasObject {
	fakeConnection := binding.BindBool(&config.UseFakeConnection)
	fakeConnection.AddListener(binding.NewDataListener(func() {
		val, err := fakeConnection.Get()
		if err != nil {
			logger.Debugf("getting fake connection value: %v", err)
			return
		}

		if val {
			openSSM2Connection = fakeOpenFunc
		} else {
			openSSM2Connection = defaultOpenFunc
		}
	}))

	form := widget.NewForm(
		widget.NewFormItem("Log Directory", widget.NewEntryWithData(
			binding.BindString(config.LogDirectory))),
		widget.NewFormItem("Log File Name Format", widget.NewEntryWithData(
			binding.BindString(config.LogFileNameFormat))),
		widget.NewFormItem("Auto Connect", widget.NewCheckWithData("",
			binding.BindBool(&config.AutoConnect))),
		widget.NewFormItem("Default to Logging Tab", widget.NewCheckWithData(
			"", binding.BindBool(&config.DefaultToLoggingTab))),
		widget.NewFormItem("Use Fake Connection", widget.NewCheckWithData("", fakeConnection)),
	)

	return form
}
