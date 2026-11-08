package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
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
	if cfg.UseFakeConnection {
		openSSM2Connection = fakeOpenFunc
	}

	app := NewApp(cfg)

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
