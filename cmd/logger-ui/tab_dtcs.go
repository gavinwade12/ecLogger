package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

const (
	labelSet    = "Set"
	labelStored = "Stored"
)

type DTCsTab struct {
	app *App

	list       *fyne.Container
	refreshBtn *widget.Button
	status     *hint
}

func NewDTCsTab(app *App) *DTCsTab {
	tab := &DTCsTab{
		app:    app,
		list:   container.New(newListLayout(theme.Padding())),
		status: newHint(""),
	}
	tab.refreshBtn = widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), tab.refresh)
	tab.status.Wrapping = fyne.TextWrapWord
	tab.status.segment.Style.Alignment = fyne.TextAlignLeading
	tab.status.Hide()
	return tab
}

func (t *DTCsTab) Container() fyne.CanvasObject {
	// the button keeps its height when a long status wraps
	header := container.NewPadded(container.NewBorder(nil, nil,
		container.NewVBox(t.refreshBtn), nil, t.status))
	return container.NewBorder(header, nil, nil, nil,
		container.NewVScroll(container.NewPadded(t.list)))
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
	t.list.RemoveAll()
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
		t.status.SetError("Couldn't read the DTCs: " + err.Error())
		t.status.Show()
		return
	}
	if len(setDTCs) == 0 && len(storedDTCs) == 0 {
		t.setStatus("No DTCs found")
		return
	}
	t.setStatus("")

	sort.Sort(sortableDTCs(setDTCs))
	sort.Sort(sortableDTCs(storedDTCs))
	widths := dtcColumnWidths()
	for _, dtc := range setDTCs {
		t.list.Objects = append(t.list.Objects, newDTCRow(widths, dtc, labelSet, theme.ColorNameWarning))
	}
	for _, dtc := range storedDTCs {
		t.list.Objects = append(t.list.Objects, newDTCRow(widths, dtc, labelStored, colorNameSecondaryText))
	}
	t.list.Refresh()
}

// The DTC row columns: its code, its description, and whether it's set or
// stored.
const dtcDescriptionColumn = 1

// dtcColumnWidths returns the widths of the code and set or stored
// columns, so any code or label fits.
func dtcColumnWidths() []float32 {
	textSize := theme.TextSize()
	code := fyne.MeasureText("P0000", textSize, fyne.TextStyle{Bold: true}).Width
	label := max(fyne.MeasureText(labelSet, textSize, fyne.TextStyle{Bold: true}).Width,
		fyne.MeasureText(labelStored, textSize, fyne.TextStyle{Bold: true}).Width)
	return []float32{code, label}
}

// newDTCRow returns a row showing dtc's code and description, and label
// in labelColor.
func newDTCRow(widths []float32, dtc ssm2.DTC, label string, labelColor fyne.ThemeColorName) *columnsRow {
	code, description := splitDTCName(dtc.Name)
	codeText := newWrappedText(code, theme.SizeNameText, theme.ColorNameForeground)
	codeText.style.Bold = true
	labelText := newWrappedText(label, theme.SizeNameText, labelColor)
	labelText.style.Bold = true
	labelText.alignment = fyne.TextAlignTrailing

	row := newColumnsRow(widths, dtcDescriptionColumn, colorNameTile,
		codeText,
		newWrappedText(description, theme.SizeNameText, theme.ColorNameForeground),
		labelText,
	)
	row.verticalInset = theme.SizeNameInnerPadding
	return row
}

// splitDTCName splits a DTC's name, like "P0102 MAF SENSOR LOW INPUT",
// into its code and description.
func splitDTCName(name string) (code, description string) {
	code, description, ok := strings.Cut(name, " ")
	if !ok || len(code) != 5 {
		return "", name
	}
	return code, description
}

type sortableDTCs []ssm2.DTC

func (a sortableDTCs) Len() int           { return len(a) }
func (a sortableDTCs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableDTCs) Less(i, j int) bool { return a[i].Name < a[j].Name }
