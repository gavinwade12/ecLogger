package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/gavinwade12/ecLogger/internal/config"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

type TabType int

const (
	TabConnection TabType = 0
	TabParameters TabType = 1
	TabLogging    TabType = 2
	TabGraphing   TabType = 3
	TabLogFiles   TabType = 4
	TabDTCs       TabType = 5
	TabSettings   TabType = 6
)

type App struct {
	config       *config.Config
	loggedParams *logging.Params

	fyneApp fyne.App
	// win is the window showing the app, for dialogs
	win fyne.Window

	tabItems      *container.AppTabs
	ConnectionTab *ConnectionTab
	ParametersTab *ParametersTab
	LoggingTab    *LoggingTab
	GraphingTab   *GraphingTab
	LogFilesTab   *LogFilesTab
	DTCsTab       *DTCsTab
	SettingsTab   *SettingsTab

	// openConnection opens a connection on a serial port, and listPorts
	// lists the available serial ports. Tests replace them with fakes.
	openConnection OpenFunc
	listPorts      func() ([]string, error)

	connection ssm2.Connection
	ecu        *ssm2.ECU
}

// OpenFunc opens a connection to the ECU on the given serial port.
type OpenFunc func(port string) (ssm2.Connection, error)

// NewApp builds the UI on fyneApp. open opens real connections; it's
// replaced with a fake connection when cfg.UseFakeConnection is set.
func NewApp(fyneApp fyne.App, cfg *config.Config, open OpenFunc, listPorts func() ([]string, error)) *App {
	app := &App{
		config:         cfg,
		loggedParams:   logging.NewParams(cfg.LoggedParams),
		fyneApp:        fyneApp,
		openConnection: open,
		listPorts:      listPorts,
	}
	fyneApp.Settings().SetTheme(newAppTheme())

	app.ConnectionTab = NewConnectionTab(app)
	app.ParametersTab = NewParametersTab(app)
	app.LoggingTab = NewLoggingTab(app)
	app.GraphingTab = NewGraphingTab(app)
	app.LogFilesTab = NewLogFilesTab(app)
	app.DTCsTab = NewDTCsTab(app)
	app.SettingsTab = NewSettingsTab(app)
	app.tabItems = container.NewAppTabs(
		container.NewTabItem("Connection", app.ConnectionTab.Container()),
		container.NewTabItem("Parameters", app.ParametersTab.Container()),
		container.NewTabItem("Logging", app.LoggingTab.Container()),
		container.NewTabItem("Graphing", app.GraphingTab.Container()),
		container.NewTabItem("Log Files", app.LogFilesTab.Container()),
		container.NewTabItem("DTCs", app.DTCsTab.Container()),
		container.NewTabItem("Settings", app.SettingsTab.Container()),
	)
	app.tabItems.OnSelected = func(item *container.TabItem) {
		if item == app.tabItems.Items[TabLogFiles] {
			app.LogFilesTab.refresh()
		}
		app.GraphingTab.setSelected(item == app.tabItems.Items[TabGraphing])
	}
	app.toggleConnectionRelatedTabs(false)
	app.tabItems.SetTabLocation(container.TabLocationLeading)

	return app
}

func (a *App) OnNewConnection(conn ssm2.Connection, ecu *ssm2.ECU) {
	a.connection = conn
	a.ecu = ecu

	a.ParametersTab.setAvailableParameters(ecu)
	a.GraphingTab.onConnect()
	a.LoggingTab.updateLiveLogParameters()
	a.toggleConnectionRelatedTabs(true)
}

func (a *App) OnDisconnect() {
	if a.connection == nil {
		return
	}
	a.LoggingTab.DisableLogging()

	a.connection.Close()
	a.connection = nil
	a.ecu = nil
	a.ConnectionTab.onDisconnect()

	a.toggleConnectionRelatedTabs(false)
	a.ParametersTab.setAvailableParameters(nil)
	a.LoggingTab.updateLiveLogParameters()
}

// SetWindow sets the window that shows the app's content, which dialogs
// are shown over.
func (a *App) SetWindow(w fyne.Window) { a.win = w }

// Content returns the app's UI, to be shown in a window.
func (a *App) Content() fyne.CanvasObject {
	return a.tabItems
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
