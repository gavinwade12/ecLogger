package main_test

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

// liveWindowSelect returns the Settings tab's Live Graph Window select.
func (ta *testApp) liveWindowSelect() *widget.Select {
	ta.t.Helper()
	var sel *widget.Select
	ta.do(func() {
		for _, s := range findAll(ta.content(ui.TabSettings), func(*widget.Select) bool { return true }) {
			if slices.Contains(s.Options, "5 minutes") {
				sel = s
			}
		}
	})
	if sel == nil {
		ta.t.Fatal("no Live Graph Window select on the Settings tab")
	}
	return sel
}

func TestSettingsLiveGraphWindow(t *testing.T) {
	ta := newTestApp(t)
	if !slices.Contains(ta.texts(ui.TabSettings), "Live Graph Window") {
		t.Errorf("no Live Graph Window setting: %v", ta.texts(ui.TabSettings))
	}
	sel := ta.liveWindowSelect()
	want := []string{"1 minute", "2 minutes", "5 minutes", "10 minutes", "15 minutes", "30 minutes"}
	if !slices.Equal(sel.Options, want) {
		t.Errorf("options = %v, want %v", sel.Options, want)
	}
	if sel.Selected != "5 minutes" {
		t.Errorf("selected %q by default, want 5 minutes", sel.Selected)
	}

	ta.do(func() { sel.SetSelected("15 minutes") })
	ta.do(func() {
		if ta.cfg.LiveGraphMinutes != 15 {
			t.Errorf("LiveGraphMinutes = %d, want 15", ta.cfg.LiveGraphMinutes)
		}
	})
}
