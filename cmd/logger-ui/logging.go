package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
)

var liveLogContainer *fyne.Container

func loggingContainer() fyne.CanvasObject {
	liveLogContainer = container.New(layout.NewGridLayout(3))
	return container.NewVScroll(liveLogContainer)
}

func updateLiveLogParameters() {
	liveLogContainer.RemoveAll()

	for id, param := range config.LoggedParams {
		if !param.LiveLog {
			continue
		}

		label := widget.NewLabel("")
		if param.Derived {
			label.SetText(ssm2.DerivedParameters[id].Name)
		} else {
			label.SetText(ssm2.Parameters[id].Name)
		}

		liveLogContainer.Objects = append(liveLogContainer.Objects,
			label)
	}

	liveLogContainer.Refresh()
}
