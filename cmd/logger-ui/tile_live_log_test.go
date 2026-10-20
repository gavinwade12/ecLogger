package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

func TestWrapText(t *testing.T) {
	test.NewTempApp(t)
	width := fyne.MeasureText("Quantity Correction", 14, fyne.TextStyle{}).Width

	for _, tt := range []struct {
		text string
		want []string
	}{
		{"", []string{""}},
		{"Engine Speed", []string{"Engine Speed"}},
		{"Quantity Correction Cylinder #1", []string{"Quantity Correction", "Cylinder #1"}},
		// a word too long for a line overflows rather than being split
		{"Roughness QuantityCorrectionCylinder", []string{"Roughness", "QuantityCorrectionCylinder"}},
	} {
		if got := wrapText(tt.text, width, 14, fyne.TextStyle{}); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("wrapText(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestLiveLogTileHeightGrowsWithItsName(t *testing.T) {
	test.NewTempApp(t).Settings().SetTheme(newAppTheme())
	short := newLiveLogTile(newLiveLogModel("P8", "Engine Speed"))
	long := newLiveLogTile(newLiveLogModel("P1", "Quantity Correction Cylinder #1"))

	if short.heightForWidth(180) >= long.heightForWidth(180) {
		t.Error("expected a name that wraps to make the tile taller")
	}
	if got, want := long.heightForWidth(1000), short.heightForWidth(1000); got != want {
		t.Errorf("height with room for the name = %v, want %v", got, want)
	}
	if got, want := long.MinSize(), fyne.NewSize(180, long.heightForWidth(180)); got != want {
		t.Errorf("MinSize = %v, want %v", got, want)
	}
}

func TestLiveLogTileShowsTheModel(t *testing.T) {
	h := newTestApp(t)
	m := newLiveLogModel("P8", "Engine Speed")
	var r *liveLogTileRenderer
	h.do(func() {
		tile := newLiveLogTile(m)
		tile.Resize(tile.MinSize())
		r = test.TempWidgetRenderer(t, tile).(*liveLogTileRenderer)
	})
	// listeners are called on the main goroutine
	drain := func() { fyne.DoAndWait(func() {}) }

	check := func(wantValue, wantUnit, wantMin, wantMax string) {
		t.Helper()
		drain()
		h.do(func() {
			got := []string{r.value.Text, r.unit.Text, r.min.Text, r.max.Text}
			if want := []string{wantValue, wantUnit, wantMin, wantMax}; !reflect.DeepEqual(got, want) {
				t.Errorf("value, unit, min, max = %q, want %q", got, want)
			}
			if r.name[0].Text != "Engine Speed" {
				t.Errorf("name = %q, want %q", r.name[0].Text, "Engine Speed")
			}
		})
	}

	check(noValue, "", noValue, noValue)
	// the session's stats cover every value so far
	m.Update(ssm2.ParameterValue{Value: 800, Unit: units.RPM}, logging.Stats{Min: 800, Max: 800, Count: 1})
	m.Update(ssm2.ParameterValue{Value: 2500, Unit: units.RPM}, logging.Stats{Min: 800, Max: 2500, Count: 2})
	check("2500.00", string(units.RPM), "800.00", "2500.00")
}
