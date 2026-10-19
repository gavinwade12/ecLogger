//go:build dev

package main

import (
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func init() {
	settingsFormItems = append(settingsFormItems,
		func(app *App) []*widget.FormItem {
			return []*widget.FormItem{
				widget.NewFormItem("Use Fake Connection", widget.NewCheckWithData("",
					binding.BindBool(&app.config.UseFakeConnection))),
			}
		})
}
