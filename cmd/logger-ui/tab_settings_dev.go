//go:build dev

package main

import (
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func init() {
	settingsFormItems = append(settingsFormItems,
		func(app *App) []*widget.FormItem {
			fakeConnection := binding.BindBool(&app.config.UseFakeConnection)
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

			return []*widget.FormItem{
				widget.NewFormItem("Use Fake Connection", widget.NewCheckWithData("", fakeConnection)),
			}
		})
}
