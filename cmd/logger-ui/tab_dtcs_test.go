package main_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// refreshDTCs taps Refresh on the DTCs tab and waits for the read to finish.
func (ta *testApp) refreshDTCs() {
	ta.t.Helper()
	ta.show(ui.TabDTCs)
	ta.tap(ui.TabDTCs, "Refresh")
	ta.waitForDTCRead()
}

func (ta *testApp) waitForDTCRead() {
	ta.t.Helper()
	ta.waitFor("the DTC refresh", func() bool {
		return !button(ta.content(ui.TabDTCs), "Refresh").Disabled()
	})
}

func TestDTCRefreshShowsDTCs(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.ecu.stubDTCs(
		[]ssm2.DTC{ssm2.DTCs["D9"]},
		[]ssm2.DTC{ssm2.DTCs["D1"], ssm2.DTCs["D10"]}, nil)

	ta.refreshDTCs()
	// set DTCs come first, then stored, each sorted by name
	want := []string{
		"P0102", "MAF SENSOR LOW INPUT", "Set",
		"P0103", "MAF SENSOR HIGH INPUT", "Stored",
		"P0335", "CRANKSHAFT POS. SENSOR A MALFUNCTION", "Stored",
	}
	got := slices.DeleteFunc(ta.texts(ui.TabDTCs), func(s string) bool { return s == "Refresh" })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DTCs tab shows %q, want %q", got, want)
	}
}

func TestDTCRefreshStatus(t *testing.T) {
	tests := map[string]struct {
		err       error
		want      string
		wantColor fyne.ThemeColorName
	}{
		"none":  {nil, "No DTCs found", theme.ColorNamePlaceHolder},
		"error": {errors.New("boom"), "Couldn't read the DTCs: reading set DTCs: sending read addresses request: boom", theme.ColorNameError},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			ta.resize(1400, 600) // so a long status doesn't wrap
			ta.connect()
			ta.ecu.stubDTCs(nil, nil, tt.err)

			ta.refreshDTCs()
			ta.do(func() {
				content := ta.content(ui.TabDTCs)
				if got := texts(content); !slices.Contains(got, tt.want) {
					t.Errorf("DTCs tab shows %q, want %q", got, tt.want)
				}
				if got, _ := textColor(content, tt.want); got != tt.wantColor {
					t.Errorf("status color = %q, want %q", got, tt.wantColor)
				}
			})
		})
	}
}

func TestDTCRefreshDoesNotBlockTheUI(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.ecu.holdDTCs()
	ta.show(ui.TabDTCs)

	// with the reads blocked, the tap must still return
	ta.tap(ui.TabDTCs, "Refresh")
	ta.do(func() {
		if !button(ta.content(ui.TabDTCs), "Refresh").Disabled() ||
			!slices.Contains(texts(ta.content(ui.TabDTCs)), "Reading DTCs...") {
			t.Error("expected Refresh disabled while reading")
		}
	})
	ta.ecu.releaseDTCs()
	ta.waitForDTCRead()
}

func TestDTCRefreshKeepsFileLogging(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.setFileLog("Engine Speed", true)
	ta.show(ui.TabLogging)
	ta.tap(ui.TabLogging, "Log To File")
	ta.waitForLiveReads()

	ta.ecu.holdDTCs()
	ta.show(ui.TabDTCs)
	ta.tap(ui.TabDTCs, "Refresh")
	// the connection is busy, so the logging session pauses
	ta.assertLiveReadsStopped()

	ta.ecu.releaseDTCs()
	ta.waitForDTCRead()
	if !ta.buttonShown(ui.TabLogging, "Stop") {
		t.Error("refreshing DTCs stopped file logging")
	}
	ta.waitForLiveReads()
	ta.tap(ui.TabLogging, "Stop")
}

func TestParameterChangesDuringDTCRefresh(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.ecu.holdDTCs()
	ta.show(ui.TabDTCs)
	ta.tap(ui.TabDTCs, "Refresh")

	// live logging a parameter mustn't start a session mid-read
	ta.setLiveLog("Engine Speed", true)
	ta.assertLiveReadsStopped()

	ta.ecu.releaseDTCs()
	ta.waitForLiveReads()
}
