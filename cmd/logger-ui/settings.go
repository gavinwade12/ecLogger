package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
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
			openSSM2Connection = func() error {
				conn = ssm2.NewFakeConnection(time.Millisecond * 50)
				return nil
			}
		} else {
			openSSM2Connection = defaultOpenFunc
		}
	}))
	form := widget.NewForm(
		widget.NewFormItem("Log File Name Format", widget.NewEntryWithData(
			binding.BindString(config.LogFileNameFormat))),
		widget.NewFormItem("Use Fake Connection", widget.NewCheckWithData("", fakeConnection)),
	)

	return form
}
