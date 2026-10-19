package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
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

		h.LoggingTab.stopFileLogging()
		for _, input := range h.ParametersTab.inputs {
			if input.Disabled() {
				t.Fatalf("%T disabled after file logging", input)
			}
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
