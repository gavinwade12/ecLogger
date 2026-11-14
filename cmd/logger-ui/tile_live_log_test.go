package main_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

const noValue = "–"

var valuePattern = regexp.MustCompile(`^-?\d+\.\d\d$`)

// tileView is what a live log tile shows.
type tileView struct {
	tile                        fyne.CanvasObject
	name, value, unit, min, max string
}

// tilesNow returns the tiles on the Logging tab. It must be called on the
// main thread.
func (ta *testApp) tilesNow() []tileView {
	shows := func(o fyne.CanvasObject) bool {
		ts := texts(o)
		return slices.Contains(ts, "min") && slices.Contains(ts, "max")
	}
	var tiles []tileView
	walk(ta.content(ui.TabLogging), func(o fyne.CanvasObject, _ []fyne.CanvasObject) {
		// a tile is the innermost object with a min and a max
		if !shows(o) || slices.ContainsFunc(childrenOf(o), shows) {
			return
		}
		ts := texts(o)
		n := len(ts)
		if n < 6 { // not laid out yet
			return
		}
		v := tileView{tile: o, max: ts[n-1], min: ts[n-3]}
		rest := ts[:n-4]
		isValue := func(s string) bool { return s == noValue || valuePattern.MatchString(s) }
		if isValue(rest[len(rest)-1]) {
			v.value, rest = rest[len(rest)-1], rest[:len(rest)-1]
		} else {
			v.unit, v.value, rest = rest[len(rest)-1], rest[len(rest)-2], rest[:len(rest)-2]
		}
		v.name = strings.Join(rest, " ")
		tiles = append(tiles, v)
	})
	return tiles
}

func (ta *testApp) tiles() (tiles []tileView) {
	ta.do(func() { tiles = ta.tilesNow() })
	return tiles
}

// tile returns the tile with the given name.
func (ta *testApp) tile(name string) (tileView, bool) {
	for _, v := range ta.tiles() {
		if v.name == name {
			return v, true
		}
	}
	return tileView{}, false
}

// waitForTile waits for the tile with the given name to match cond.
func (ta *testApp) waitForTile(what, name string, cond func(tileView) bool) tileView {
	ta.t.Helper()
	var found tileView
	ta.waitFor(what, func() bool {
		for _, v := range ta.tilesNow() {
			if v.name == name && cond(v) {
				found = v
				return true
			}
		}
		return false
	})
	return found
}

func hasValue(v tileView) bool { return v.value != noValue }

func TestLiveLogTileShowsPlaceholdersUntilAValue(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.ecu.holdLive()
	ta.setLiveLog("Engine Speed", true)
	ta.show(ui.TabLogging)

	// a placeholder rather than 0, which looks like a real reading
	v := ta.waitForTile("the tile", "Engine Speed", func(tileView) bool { return true })
	if v.value != noValue || v.min != noValue || v.max != noValue || v.unit != "" {
		t.Errorf("tile shows value %q, unit %q, min %q, max %q; want placeholders and no unit",
			v.value, v.unit, v.min, v.max)
	}

	ta.ecu.releaseLive()
	v = ta.waitForTile("a value", "Engine Speed", hasValue)
	if v.unit != "rpm" {
		t.Errorf("unit = %q, want rpm", v.unit)
	}
}

func TestLiveLogTileShowsTheSessionsMinAndMax(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.setLiveLog("Vehicle Speed", true)
	ta.show(ui.TabLogging)

	// the min and max cover every value so far, so they only widen
	float := func(s string) float64 {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatalf("%q isn't a number", s)
		}
		return f
	}
	ta.waitForTile("a value", "Vehicle Speed", hasValue)
	var prev tileView
	widened := false
	ta.waitFor("the min and max to differ", func() bool {
		for _, v := range ta.tilesNow() {
			if !hasValue(v) || v.min == noValue {
				continue
			}
			value, lo, hi := float(v.value), float(v.min), float(v.max)
			if value < lo || value > hi {
				t.Errorf("value %v isn't between the min %v and max %v", value, lo, hi)
			}
			if prev.min != "" && (lo > float(prev.min) || hi < float(prev.max)) {
				t.Errorf("min and max narrowed from %v..%v to %v..%v", prev.min, prev.max, lo, hi)
			}
			widened = widened || lo != hi
			prev = v
		}
		return widened
	})
}

func TestLiveLogTileValueDoesNotMoveTheUnit(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.ecu.answerLiveWith(0)
	ta.setLiveLog("Vehicle Speed", true)
	ta.show(ui.TabLogging)

	// the unit's position and the value's right edge, once the tile shows
	// the value
	positions := func(b byte) (unit fyne.Position, valueRight float32) {
		t.Helper()
		ta.ecu.answerLiveWith(b)
		want := strconv.Itoa(int(b)) + ".00"
		v := ta.waitForTile("the value "+want, "Vehicle Speed", func(v tileView) bool { return v.value == want })
		ta.do(func() {
			for _, text := range findAll[*canvas.Text](v.tile, nil) {
				switch text.Text {
				case v.unit:
					unit = text.Position()
				case v.value:
					valueRight = text.Position().X + text.Size().Width
				}
			}
		})
		return unit, valueRight
	}

	unit, valueRight := positions(0)
	// with 1, 2 and 3 digits
	for _, b := range []byte{5, 42, 150, 255} {
		if u, right := positions(b); u != unit || right != valueRight {
			t.Errorf("showing %d moved the unit to %v and the value's right edge to %v, want %v and %v",
				b, u, right, unit, valueRight)
		}
	}
}

func TestLiveLogTilesWrapLongNames(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	const long = "Quantity Correction Cylinder #1"
	ta.setLiveLog("Engine Speed", true)
	ta.setLiveLog(long, true)
	ta.show(ui.TabLogging)

	// in one column a tile is as wide as the window leaves it
	height := func(name string) float32 {
		v := ta.waitForTile("the "+name+" tile", name, func(tileView) bool { return true })
		var h float32
		ta.do(func() { h = v.tile.Size().Height })
		return h
	}
	ta.resize(260, 600)
	if short, wrapped := height("Engine Speed"), height(long); wrapped <= short {
		t.Errorf("tile for a name that wraps is %v tall, want taller than %v", wrapped, short)
	}

	// with room for the name, it's on one line, and the tiles are the same height
	ta.resize(400, 600)
	if short, one := height("Engine Speed"), height(long); one != short {
		t.Errorf("tile for a name with room is %v tall, want %v", one, short)
	}
	ta.do(func() {
		if !slices.Contains(texts(ta.content(ui.TabLogging)), long) {
			t.Errorf("Logging tab shows %q, want the name on one line", texts(ta.content(ui.TabLogging)))
		}
	})
}
