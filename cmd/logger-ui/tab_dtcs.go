package main

import (
	"context"
	"fmt"
	"sort"
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

	grid       *fyne.Container
	refreshBtn *widget.Button
	status     *widget.Label
}

func NewDTCsTab(app *App) *DTCsTab {
	tab := &DTCsTab{
		app:    app,
		grid:   container.NewGridWithColumns(2),
		status: widget.NewLabel(""),
	}
	tab.refreshBtn = widget.NewButton("Refresh", tab.refresh)
	tab.status.Hide()
	return tab
}

func (t *DTCsTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.NewVBox(
		t.refreshBtn,
		t.status,
		t.grid,
	))
}

func (t *DTCsTab) setStatus(s string) {
	t.status.SetText(s)
	if s == "" {
		t.status.Hide()
	} else {
		t.status.Show()
	}
}

// refresh reads the DTCs on another goroutine so the UI stays responsive.
// The logging session is paused while reading, since it uses the same
// connection, but file logging stays on and resumes with it.
func (t *DTCsTab) refresh() {
	conn := t.app.connection
	if conn == nil {
		return
	}

	t.refreshBtn.Disable()
	t.grid.RemoveAll()
	t.setStatus("Reading DTCs...")
	t.app.LoggingTab.PauseLogging()

	go func() {
		setDTCs, storedDTCs, err := t.readAll(conn)
		fyne.Do(func() {
			t.app.LoggingTab.ResumeLogging()
			t.refreshBtn.Enable()
			t.showDTCs(setDTCs, storedDTCs, err)
		})
	}()
}

func (t *DTCsTab) readAll(conn ssm2.Connection) (setDTCs, storedDTCs []ssm2.DTC, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if setDTCs, err = ssm2.ReadSetDTCs(ctx, conn); err != nil {
		return nil, nil, fmt.Errorf("reading set DTCs: %w", err)
	}
	if storedDTCs, err = ssm2.ReadStoredDTCs(ctx, conn); err != nil {
		return nil, nil, fmt.Errorf("reading stored DTCs: %w", err)
	}
	return setDTCs, storedDTCs, nil
}

func (t *DTCsTab) showDTCs(setDTCs, storedDTCs []ssm2.DTC, err error) {
	if err != nil {
		logger.Debug(err.Error())
		t.setStatus("Couldn't read the DTCs: " + err.Error())
		return
	}
	if len(setDTCs) == 0 && len(storedDTCs) == 0 {
		t.setStatus("No DTCs found")
		return
	}
	t.setStatus("")

	sort.Sort(sortableDTCs(setDTCs))
	sort.Sort(sortableDTCs(storedDTCs))
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

type sortableDTCs []ssm2.DTC

func (a sortableDTCs) Len() int           { return len(a) }
func (a sortableDTCs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableDTCs) Less(i, j int) bool { return a[i].Name < a[j].Name }
