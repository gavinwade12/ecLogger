package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

const (
	configDirectoryName             = "ssm2"
	configFileName                  = ".ssm2"
	defaultLogFileNameFormat string = "ssm2_log_{{romId}}_{{timestamp}}.csv"
)

var logger = ssm2.DefaultLogger(os.Stdout)

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	app := NewApp(config)

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

	if err := saveConfig(*app.config); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() (*Config, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("finding user home directory: %w", err)
	}
	ssm2Dir := filepath.Join(dir, configDirectoryName)

	var config Config
	f, err := os.Open(filepath.Join(ssm2Dir, configFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("opening config file: %w", err)
		}

		logDirectory := filepath.Join(ssm2Dir, "logs")
		config.LogDirectory = &logDirectory
		fileNameFormat := defaultLogFileNameFormat
		config.LogFileNameFormat = &fileNameFormat
		config.LoggedParams = make(map[string]*LoggedParam)
		return &config, nil
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("decoding config from file: %w", err)
	}
	if config.LoggedParams == nil {
		config.LoggedParams = make(map[string]*LoggedParam)
	}
	if config.UseFakeConnection {
		openSSM2Connection = fakeOpenFunc
	}
	if config.LogDirectory == nil {
		logDirectory := filepath.Join(ssm2Dir, "logs")
		config.LogDirectory = &logDirectory
	}
	if config.LogFileNameFormat == nil {
		fileNameFormat := defaultLogFileNameFormat
		config.LogFileNameFormat = &fileNameFormat
	}
	return &config, nil
}

func saveConfig(config Config) error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding user home directory: %w", err)
	}

	configPath := filepath.Join(dir, configDirectoryName, configFileName)
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("opening config file: %w", err)
		}

		if err = os.Mkdir(filepath.Join(dir, configDirectoryName), os.ModePerm); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
		f, err = os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			return fmt.Errorf("opening config file: %w", err)
		}
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(config)
	if err != nil {
		return fmt.Errorf("encoding config to file: %w", err)
	}
	return nil
}
