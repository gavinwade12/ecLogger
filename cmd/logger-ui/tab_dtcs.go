package main

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

const (
	labelSet    = "Set"
	labelStored = "Stored"
)

type DTCsTab struct {
	app *App

	grid *fyne.Container
}

func NewDTCsTab(app *App) *DTCsTab {
	tab := &DTCsTab{
		app:  app,
		grid: container.NewGridWithColumns(2),
	}
	return tab
}

func (t *DTCsTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.NewVBox(
		widget.NewButton("Refresh", t.refresh),
		t.grid,
	))
}

func (t *DTCsTab) refresh() {
	t.grid.RemoveAll()

	if t.app.connection == nil {
		return
	}

	t.app.LoggingTab.DisableLogging()
	defer t.app.LoggingTab.EnableLogging()

	setDTCs, err := t.readDTCs(false)
	if err != nil {
		logger.Debug(err.Error())
		return
	}

	storedDTCs, err := t.readDTCs(true)
	if err != nil {
		logger.Debug(err.Error())
		return
	}

	for _, dtc := range setDTCs {
		t.grid.Objects = append(t.grid.Objects,
			NewWrappedLabel(dtc.Name),
			widget.NewLabel(labelSet),
		)
	}

	for _, dtc := range storedDTCs {
		t.grid.Objects = append(t.grid.Objects,
			NewWrappedLabel(dtc.Name),
			widget.NewLabel(labelStored),
		)
	}

	t.grid.Refresh()
}

func (t *DTCsTab) readDTCs(stored bool) ([]ssm2.DTC, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if stored {
		return ssm2.ReadStoredDTCs(ctx, t.app.connection)
	}
	return ssm2.ReadSetDTCs(ctx, t.app.connection)
}
