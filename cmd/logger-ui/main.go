package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.bug.st/serial/enumerator"
)

var (
	configFileName           = ".ssm2"
	defaultLogFileNameFormat = "ssm2_log_{{romId}}_{{timestamp}}.csv"
)

func init() {
	mustLoadConfig()
}

var config struct {
	SelectedPort      string
	LogFileNameFormat *string
}

func main() {
	pl, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}

	ports := make([]string, len(pl))
	for i, p := range pl {
		ports[i] = p.Name
	}

	a := app.New()
	w := a.NewWindow("Logger")
	w.Resize(fyne.NewSize(800, 400))

	tabs := container.NewAppTabs(
		container.NewTabItem("Connection", connectionContainer()),
		container.NewTabItem("Parameters", widget.NewLabel("Parameters")),
		container.NewTabItem("Logging", widget.NewLabel("Hello!")),
		container.NewTabItem("Settings", settingsContainer()),
	)

	tabs.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabs)
	w.ShowAndRun()
}

func strPtr(s string) *string {
	return &s
}

func mustLoadConfig() {
	dir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("finding user home directory: %v", err))
	}

	f, err := os.Open(filepath.Join(dir, configFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			panic(fmt.Sprintf("opening config file: %v", err))
		}

		config.LogFileNameFormat = &defaultLogFileNameFormat
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		panic(fmt.Sprintf("decoding config from file: %v", err))
	}
}
