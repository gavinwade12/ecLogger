package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/units"
)

func TestUnitOptions(t *testing.T) {
	want := []string{"kPa", "bar", "hPa", "inHg", "mmHg", "psi"}
	for i := 0; i < 20; i++ { // map iteration order varies between runs
		if got := unitOptions(units.KPA); !reflect.DeepEqual(got, want) {
			t.Fatalf("unitOptions(kPa) = %v, want %v", got, want)
		}
	}
	if got := unitOptions(units.Gear); !reflect.DeepEqual(got, []string{"gear"}) {
		t.Fatalf("unitOptions(gear) = %v, want [gear]", got)
	}
}

func TestFileLoggingDisablesParameterChanges(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	h.do(func() {
		if len(h.ParametersTab.inputs) == 0 {
			t.Fatal("expected parameter inputs")
		}
		h.loggedParams.SetLogToFile("P8", false, units.RPM, true)
		h.LoggingTab.startFileLogging()
		for _, input := range h.ParametersTab.inputs {
			if !input.Disabled() {
				t.Fatalf("%T enabled while file logging", input)
			}
		}
		if !h.ParametersTab.locked.Visible() {
			t.Error("expected a hint saying why parameters can't be changed")
		}

		h.LoggingTab.stopFileLogging()
		for _, input := range h.ParametersTab.inputs {
			if input.Disabled() {
				t.Fatalf("%T disabled after file logging", input)
			}
		}
		if h.ParametersTab.locked.Visible() {
			t.Error("expected no hint once parameters can be changed")
		}
	})
}

func TestLiveLogCheckLogsParameter(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	h.do(func() {
		check := h.ParametersTab.inputs[1].(*widget.Check) // the first parameter's Live Log
		test.Tap(check)

		if h.loggedParams.Len() != 1 {
			t.Fatalf("%d logged parameters, want 1", h.loggedParams.Len())
		}
		if len(h.LoggingTab.liveLogModels) != 1 || !h.LoggingTab.runner.Running() {
			t.Fatal("expected the parameter to be live logged")
		}

		test.Tap(check)
		if h.loggedParams.Len() != 0 || len(h.LoggingTab.liveLogModels) != 0 || h.LoggingTab.runner.Running() {
			t.Fatal("expected the parameter to no longer be logged")
		}
	})
}

func TestUnitChangeResetsLiveLogMinMax(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	var id string
	var unit *widget.Select
	h.do(func() {
		// inputs are Log To File, Live Log and unit for each parameter
		for i := 0; i < len(h.ParametersTab.inputs); i += 3 {
			if s := h.ParametersTab.inputs[i+2].(*widget.Select); s.Selected == string(units.MPH) {
				test.Tap(h.ParametersTab.inputs[i+1].(*widget.Check))
				unit = s
				break
			}
		}
		if unit == nil {
			t.Fatal("no parameter in mph")
		}
		id = h.LoggingTab.liveLogModels[0].Id
	})

	// let the min/max build up from the fake connection's values
	var before logging.Stats
	h.waitFor("some live log values", func() bool {
		before, _ = h.LoggingTab.runner.Stats(id)
		return before.Count >= 20
	})

	h.do(func() {
		unit.SetSelected(string(units.KMH))

		// the min/max come from the session, so the unit change must have
		// started a new one rather than keep comparing against mph values
		if after, _ := h.LoggingTab.runner.Stats(id); after.Count >= before.Count {
			t.Errorf("min/max kept %d values across the unit change, want a fresh session", after.Count)
		}
	})
}

func TestParameterColumnsFitTheirUnits(t *testing.T) {
	test.NewTempApp(t)
	widths := columnWidths([][]string{{"V"}, {"raw ecu value"}})
	if len(widths) != 3 {
		t.Fatalf("got %d column widths, want 3", len(widths))
	}

	text := fyne.MeasureText("raw ecu value", theme.TextSize(), fyne.TextStyle{}).Width
	if widths[2] < text+theme.InnerPadding()*2 {
		t.Errorf("unit column is %v wide, too narrow for %q (%v)", widths[2], "raw ecu value", text)
	}
	for i, heading := range []string{headingLogToFile, headingLiveLog} {
		if w := fyne.MeasureText(heading, theme.TextSize(), fyne.TextStyle{}).Width; widths[i] < w {
			t.Errorf("%q column is %v wide, want at least %v", heading, widths[i], w)
		}
	}
}
