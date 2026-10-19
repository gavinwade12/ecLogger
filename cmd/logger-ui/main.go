package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"github.com/gavinwade12/ecLogger/internal/config"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

var logger = ssm2.DefaultLogger(os.Stdout)

func main() {
	configPath, err := config.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// every widget update from a goroutine goes through fyne.Do, so Fyne
	// doesn't need to check which goroutine calls it
	fyneApp.SetMetadata(fyne.AppMetadata{
		Name:       "Logger",
		Migrations: map[string]bool{"fyneDo": true},
	})
	app := NewApp(fyneApp.New(), cfg, openSerialConnection, listSerialPorts)

	window := app.fyneApp.NewWindow("Logger")
	window.Resize(fyne.NewSize(800, 400))
	window.SetContent(app.tabItems)

	if app.config.AutoConnect {
		app.fyneApp.Lifecycle().SetOnStarted(app.ConnectionTab.OnConnectTapped)
	}
	if app.config.DefaultToLoggingTab {
		app.SelectTab(TabLogging)
	}

	window.ShowAndRun()

	app.config.LoggedParams = app.loggedParams.Snapshot()
	if err := config.Save(configPath, app.config); err != nil {
		log.Fatal(err)
	}
}
