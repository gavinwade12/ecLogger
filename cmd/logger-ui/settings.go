package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func settingsContainer() fyne.CanvasObject {
	form := widget.NewForm(
		widget.NewFormItem("Log File Name Format", widget.NewEntryWithData(
			binding.BindString(config.LogFileNameFormat))),
	)

	return form
}
