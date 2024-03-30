package main

import (
	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

type TabType int

const (
	TabConnection TabType = 0
	TabParameters TabType = 1
	TabLogging    TabType = 2
	TabDTCs       TabType = 3
	TabSettings   TabType = 4
)

type Config struct {
	SelectedPort        string
	LogDirectory        *string
	LogFileNameFormat   *string
	LoggedParams        map[string]*LoggedParam
	UseFakeConnection   bool
	AutoConnect         bool
	DefaultToLoggingTab bool
}

type App struct {
	config       *Config
	loggedParams *LoggedParams

	fyneApp fyne.App

	tabItems      *container.AppTabs
	ConnectionTab *ConnectionTab
	ParametersTab *ParametersTab
	LoggingTab    *LoggingTab
	DTCsTab       *DTCsTab
	SettingsTab   *SettingsTab

	connection ssm2.Connection
	ecu        *ssm2.ECU
}

func NewApp(config *Config) *App {
	app := &App{
		config:       config,
		loggedParams: NewLoggedParams(config.LoggedParams),
		fyneApp:      fyneApp.New(),
	}

	app.ConnectionTab = NewConnectionTab(app)
	app.ParametersTab = NewParametersTab(app)
	app.LoggingTab = NewLoggingTab(app)
	app.DTCsTab = NewDTCsTab(app)
	app.SettingsTab = NewSettingsTab(app)
	app.tabItems = container.NewAppTabs(
		container.NewTabItem("Connection", app.ConnectionTab.Container()),
		container.NewTabItem("Parameters", app.ParametersTab.Container()),
		container.NewTabItem("Logging", app.LoggingTab.Container()),
		container.NewTabItem("DTCs", app.DTCsTab.Container()),
		container.NewTabItem("Settings", app.SettingsTab.Container()),
	)
	app.toggleConnectionRelatedTabs(false)
	app.tabItems.SetTabLocation(container.TabLocationLeading)

	return app
}

func (a *App) OnNewConnection(conn ssm2.Connection, ecu *ssm2.ECU) {
	a.connection = conn
	a.ecu = ecu

	a.ParametersTab.setAvailableParameters(ecu)
	a.LoggingTab.updateLiveLogParameters()
	a.toggleConnectionRelatedTabs(true)
}

func (a *App) OnDisconnect() {
	if a.LoggingTab.cancelLogging != nil {
		a.LoggingTab.cancelLogging()
		a.LoggingTab.cancelLogging = nil
	}

	a.connection.Close()
	a.connection = nil
	a.ecu = nil
	a.ConnectionTab.onDisconnect()

	a.toggleConnectionRelatedTabs(false)
	a.ParametersTab.setAvailableParameters(nil)
	a.LoggingTab.updateLiveLogParameters()
}

func (a *App) Connection() ssm2.Connection {
	return a.connection
}

func (a *App) EnableTab(tab TabType) {
	a.tabItems.EnableIndex(int(tab))
}

func (a *App) DisableTab(tab TabType) {
	a.tabItems.DisableIndex(int(tab))
}

func (a *App) SelectTab(tab TabType) {
	a.tabItems.SelectIndex(int(tab))
}

func (a *App) toggleConnectionRelatedTabs(enabled bool) {
	for _, tab := range []TabType{TabParameters, TabLogging, TabDTCs} {
		if enabled {
			a.EnableTab(tab)
		} else {
			a.DisableTab(tab)
		}
	}
}
