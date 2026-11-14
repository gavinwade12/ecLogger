package main_test

import (
	"testing"

	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

var connectionTabs = []ui.TabType{ui.TabParameters, ui.TabLogging, ui.TabDTCs}

func TestConnectEnablesTabs(t *testing.T) {
	ta := newTestApp(t)
	for _, tab := range connectionTabs {
		if ta.tabEnabled(tab) {
			t.Errorf("%s tab enabled before connecting", tabName(tab))
		}
	}

	ta.connect()
	for _, tab := range connectionTabs {
		if !ta.tabEnabled(tab) {
			t.Errorf("%s tab disabled after connecting", tabName(tab))
		}
	}
	if ta.buttonShown(ui.TabConnection, "Connect") || !ta.buttonShown(ui.TabConnection, "Disconnect") {
		t.Error("expected Disconnect in place of Connect")
	}
}

func TestDisconnect(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()

	ta.tap(ui.TabConnection, "Disconnect")
	ta.waitForStatus("Disconnected")
	if !ta.buttonShown(ui.TabConnection, "Connect") || ta.buttonShown(ui.TabConnection, "Disconnect") {
		t.Error("expected Connect in place of Disconnect")
	}
	ta.do(func() {
		if button(ta.content(ui.TabConnection), "Connect").Disabled() {
			t.Error("expected Connect to be enabled")
		}
		if ta.Connection() != nil {
			t.Error("expected the connection to be cleared")
		}
		// disconnecting twice must not panic
		ta.OnDisconnect()
	})
	for _, tab := range connectionTabs {
		if ta.tabEnabled(tab) {
			t.Errorf("%s tab enabled after disconnecting", tabName(tab))
		}
	}
}
