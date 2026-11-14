package main_test

import (
	"regexp"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// assertStacked checks that rows are stacked in a list gap apart, each at
// the list's full width.
func assertStacked(t *testing.T, list fyne.CanvasObject, rows []fyne.CanvasObject, gap float32) {
	t.Helper()
	if len(rows) < 2 {
		t.Fatalf("got %d rows, want several to compare", len(rows))
	}
	var y float32
	for i, row := range rows {
		if row.Position().X != 0 || row.Size().Width != list.Size().Width {
			t.Errorf("row %d at x=%v and %v wide, want the list's %v width", i, row.Position().X, row.Size().Width, list.Size().Width)
		}
		if row.Position().Y != y {
			t.Errorf("row %d at y=%v, want %v", i, row.Position().Y, y)
		}
		if row.Size().Height <= 0 {
			t.Errorf("row %d is %v tall", i, row.Size().Height)
		}
		y += row.Size().Height + gap
	}
}

// listOf returns the container in root that's a list of rows, which are all
// the objects that satisfy isRow.
func listOf(t *testing.T, root fyne.CanvasObject, isRow func(fyne.CanvasObject) bool) (list fyne.CanvasObject, rows []fyne.CanvasObject) {
	t.Helper()
	lists := findAll(root, func(c *fyne.Container) bool {
		return len(c.Objects) > 1 && !slices.ContainsFunc(c.Objects, func(o fyne.CanvasObject) bool { return !isRow(o) })
	})
	if len(lists) == 0 {
		t.Fatal("found no list of rows")
	}
	// the outermost, in case rows hold lists of their own
	return lists[0], lists[0].Objects
}

func TestDTCsAreStackedWithAGap(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.ecu.stubDTCs(
		[]ssm2.DTC{ssm2.DTCs["D9"]},
		[]ssm2.DTC{ssm2.DTCs["D1"], ssm2.DTCs["D10"]}, nil)
	ta.refreshDTCs()

	ta.do(func() {
		code := regexp.MustCompile(`^P\d{4}$`)
		list, rows := listOf(t, ta.content(ui.TabDTCs), func(o fyne.CanvasObject) bool {
			return slices.ContainsFunc(texts(o), code.MatchString)
		})
		assertStacked(t, list, rows, theme.Padding())
	})
}

func TestParametersAreStackedWithoutAGap(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.show(ui.TabParameters)

	ta.do(func() {
		list, rows := listOf(t, ta.content(ui.TabParameters), func(o fyne.CanvasObject) bool {
			return len(findAll[*widget.Select](o, nil)) == 1
		})
		assertStacked(t, list, rows, 0)
	})
}

func TestParameterColumnsLineUp(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.show(ui.TabParameters)

	ta.do(func() {
		_, rows := listOf(t, ta.content(ui.TabParameters), func(o fyne.CanvasObject) bool {
			return len(findAll[*widget.Select](o, nil)) == 1
		})
		gap := theme.InnerPadding() * 2

		// every row's columns are where the first row's are, and the name
		// takes the width the others leave
		var first []fyne.CanvasObject
		for i, r := range rows {
			cells := findAll(r, func(c *fyne.Container) bool { return len(c.Objects) == 4 })[0]
			if i == 0 {
				first = cells.Objects
			}
			c := cells.Objects
			if c[0].Position().X != 0 {
				t.Errorf("row %d: name at x=%v, want 0", i, c[0].Position().X)
			}
			if c[3].Position().X+c[3].Size().Width != cells.Size().Width {
				t.Errorf("row %d: unit column ends at %v, want the row's %v width", i, c[3].Position().X+c[3].Size().Width, cells.Size().Width)
			}
			for j := 1; j < 4; j++ {
				if c[j].Position().X != first[j].Position().X || c[j].Size().Width != first[j].Size().Width {
					t.Errorf("row %d: column %d at x=%v, %v wide; want x=%v, %v wide like the first row",
						i, j, c[j].Position().X, c[j].Size().Width, first[j].Position().X, first[j].Size().Width)
				}
				if got, want := c[j].Position().X, c[j-1].Position().X+c[j-1].Size().Width+gap; got != want {
					t.Errorf("row %d: column %d at x=%v, want %v after the column before it", i, j, got, want)
				}
			}
		}
	})
}

func TestParameterColumnsFitTheirContents(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.show(ui.TabParameters)

	ta.do(func() {
		content := ta.content(ui.TabParameters)
		// headings aren't wrapped
		for _, heading := range []string{"Parameter", "Log To File", "Live Log", "Unit"} {
			if !slices.Contains(texts(content), heading) {
				t.Errorf("Parameters tab shows %q, want the heading %q on one line", texts(content), heading)
			}
		}

		// a unit's column is wide enough for the widest unit, in whichever
		// unit it's showing
		for _, s := range findAll[*widget.Select](content, nil) {
			selected := s.Selected
			for _, option := range s.Options {
				s.Selected = option
				s.Refresh()
				if min, size := s.MinSize().Width, s.Size().Width; min > size {
					t.Fatalf("a select showing %q needs %v, but its column is %v wide", option, min, size)
				}
			}
			s.Selected = selected
			s.Refresh()
		}
	})
}

func TestFormsStayAReadableWidth(t *testing.T) {
	ta := newTestApp(t)
	const maxWidth = 560

	width := func(tab ui.TabType, find func(fyne.CanvasObject) fyne.CanvasObject) (w float32) {
		ta.show(tab)
		ta.do(func() { w = find(ta.content(tab)).Size().Width })
		return w
	}
	connectButton := func(root fyne.CanvasObject) fyne.CanvasObject { return button(root, "Connect") }
	settingsForm := func(root fyne.CanvasObject) fyne.CanvasObject { return findAll[*widget.Form](root, nil)[0] }

	ta.resize(2000, 600)
	if got := width(ui.TabConnection, connectButton); got != maxWidth {
		t.Errorf("Connection form is %v wide on a wide window, want %v", got, maxWidth)
	}
	if got := width(ui.TabSettings, settingsForm); got != maxWidth {
		t.Errorf("Settings form is %v wide on a wide window, want %v", got, maxWidth)
	}

	ta.resize(500, 600)
	if got := width(ui.TabConnection, connectButton); got <= 0 || got >= maxWidth {
		t.Errorf("Connection form is %v wide on a narrow window, want it to shrink below %v", got, maxWidth)
	}
	if got := width(ui.TabSettings, settingsForm); got <= 0 || got >= maxWidth {
		t.Errorf("Settings form is %v wide on a narrow window, want it to shrink below %v", got, maxWidth)
	}
}
