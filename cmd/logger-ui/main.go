package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/pkg/errors"
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
		go app.ConnectionTab.OnConnectTapped()
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
		return nil, errors.Wrap(err, "finding user home directory")
	}
	ssm2Dir := filepath.Join(dir, configDirectoryName)

	var config Config
	f, err := os.Open(filepath.Join(ssm2Dir, configFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "opening config file")
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
		return nil, errors.Wrap(err, "decoding config from file")
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
		return errors.Wrap(err, "finding user home directory")
	}

	configPath := filepath.Join(dir, configDirectoryName, configFileName)
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "opening config file")
		}

		if err = os.Mkdir(filepath.Join(dir, configDirectoryName), os.ModePerm); err != nil {
			return errors.Wrap(err, "creating config directory")
		}
		f, err = os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			return errors.Wrap(err, "opening config file")
		}
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(config)
	if err != nil {
		return errors.Wrap(err, "encoding config to file")
	}
	return nil
}
