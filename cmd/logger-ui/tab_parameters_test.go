package main_test

import (
	"reflect"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

func TestUnitOptions(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.show(ui.TabParameters)

	// a parameter can be shown in its default unit, then the units it
	// converts to in a stable order. The order comes from a map, so it
	// would vary between parameters if it weren't stable.
	var kpa, gear int
	ta.do(func() {
		for _, s := range findAll[*widget.Select](ta.content(ui.TabParameters), nil) {
			switch s.Selected {
			case "kPa":
				kpa++
				if want := []string{"kPa", "bar", "hPa", "inHg", "mmHg", "psi"}; !reflect.DeepEqual(s.Options, want) {
					t.Fatalf("options of a kPa parameter = %v, want %v", s.Options, want)
				}
			case "gear":
				gear++
				if want := []string{"gear"}; !reflect.DeepEqual(s.Options, want) {
					t.Fatalf("options of a gear parameter = %v, want %v", s.Options, want)
				}
			}
		}
	})
	if kpa < 2 || gear == 0 {
		t.Errorf("found %d kPa and %d gear parameters, want several of each to compare", kpa, gear)
	}
}

// parameterInputs returns the checks and selects on the Parameters tab. It
// must be called on the main thread.
func (ta *testApp) parameterInputs() (inputs []fyne.Disableable) {
	walk(ta.content(ui.TabParameters), func(o fyne.CanvasObject, _ []fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Check:
			inputs = append(inputs, v)
		case *widget.Select:
			inputs = append(inputs, v)
		}
	})
	return inputs
}

func TestFileLoggingDisablesParameterChanges(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	const hint = "Parameters can't be changed while logging to file"

	ta.setFileLog("Engine Speed", true)
	ta.do(func() {
		if len(ta.parameterInputs()) == 0 {
			t.Fatal("expected parameter inputs")
		}
	})
	ta.show(ui.TabLogging)
	ta.tap(ui.TabLogging, "Log To File")
	ta.show(ui.TabParameters)
	ta.do(func() {
		for _, input := range ta.parameterInputs() {
			if !input.Disabled() {
				t.Fatalf("%T enabled while file logging", input)
			}
		}
		if !slices.Contains(texts(ta.content(ui.TabParameters)), hint) {
			t.Error("expected a hint saying why parameters can't be changed")
		}
	})

	ta.show(ui.TabLogging)
	ta.tap(ui.TabLogging, "Stop")
	ta.show(ui.TabParameters)
	ta.do(func() {
		for _, input := range ta.parameterInputs() {
			if input.Disabled() {
				t.Fatalf("%T disabled after file logging", input)
			}
		}
		if slices.Contains(texts(ta.content(ui.TabParameters)), hint) {
			t.Error("expected no hint once parameters can be changed")
		}
	})
}

func TestLiveLogCheckLogsParameter(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()

	ta.setLiveLog("Engine Speed", true)
	ta.show(ui.TabLogging)
	ta.waitForTile("a tile", "Engine Speed", func(tileView) bool { return true })
	if n := len(ta.tiles()); n != 1 {
		t.Fatalf("%d tiles, want one for Engine Speed", n)
	}
	ta.waitForLiveReads()

	ta.setLiveLog("Engine Speed", false)
	ta.show(ui.TabLogging)
	if n := len(ta.tiles()); n != 0 {
		t.Errorf("%d tiles after unchecking Live Log, want none", n)
	}
	ta.assertLiveReadsStopped()
}

func TestUnitChangeResetsLiveLogMinMax(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.setLiveLog("Vehicle Speed", true)
	unit := ta.paramRow("Vehicle Speed").unit
	ta.show(ui.TabLogging)

	// let the min and max build up from the fake connection's values
	v := ta.waitForTile("a min and max", "Vehicle Speed", func(v tileView) bool { return v.min != v.max && v.min != noValue })
	if v.unit != "km/h" {
		t.Fatalf("unit = %q, want km/h", v.unit)
	}

	// with nothing read since the change, a tile that has reset shows no
	// value, rather than comparing against the old unit's values
	ta.ecu.holdLive()
	ta.show(ui.TabParameters)
	ta.do(func() { unit.SetSelected("mph") })
	ta.show(ui.TabLogging)
	v = ta.waitForTile("the tile", "Vehicle Speed", func(tileView) bool { return true })
	if v.value != noValue || v.min != noValue || v.max != noValue {
		t.Errorf("tile shows value %q, min %q, max %q after the unit change, want a fresh session", v.value, v.min, v.max)
	}

	ta.ecu.releaseLive()
	v = ta.waitForTile("a value in the new unit", "Vehicle Speed", hasValue)
	if v.unit != "mph" {
		t.Errorf("unit = %q, want mph", v.unit)
	}
}
