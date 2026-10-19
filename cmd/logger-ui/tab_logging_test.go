package main

import (
	"testing"

	"github.com/gavinwade12/ecLogger/units"
)

func TestEnableLoggingWithoutParams(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	h.do(func() {
		h.LoggingTab.EnableLogging()
		if h.LoggingTab.runner.Running() {
			t.Fatal("started a session with no logged parameters")
		}

		h.loggedParams.SetLiveLog("P8", false, units.RPM, true)
		h.LoggingTab.EnableLogging()
		if !h.LoggingTab.runner.Running() {
			t.Fatal("expected a session for the logged parameter")
		}

		h.loggedParams.SetLiveLog("P8", false, units.RPM, false)
		h.LoggingTab.EnableLogging()
		if h.LoggingTab.runner.Running() {
			t.Fatal("expected the session to stop once nothing is logged")
		}
	})
}
