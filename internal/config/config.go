// Package config loads and saves the logger UI's settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gavinwade12/ecLogger/internal/logging"
)

const (
	directoryName = "ssm2"
	fileName      = ".ssm2"

	// DefaultLogFileNameFormat is the log file name format used when none is configured.
	DefaultLogFileNameFormat = "ssm2_log_{{romId}}_{{timestamp}}.csv"

	// DefaultLiveGraphMinutes is how much history the live graph shows
	// unless one is configured.
	DefaultLiveGraphMinutes = 5
)

// Config holds the logger UI's settings. It's stored as JSON.
type Config struct {
	SelectedPort        string
	LogDirectory        string
	LogFileNameFormat   string
	LoggedParams        map[string]logging.LoggedParam
	UseFakeConnection   bool
	AutoConnect         bool
	DefaultToLoggingTab bool
	// LiveGraphMinutes is how much recent history the Graphing tab shows
	// while connected.
	LiveGraphMinutes int
}

// LiveGraphWindow returns how much history the live graph shows.
func (c *Config) LiveGraphWindow() time.Duration {
	return time.Duration(c.LiveGraphMinutes) * time.Minute
}

// DefaultPath returns the default config file path: ~/ssm2/.ssm2.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding user home directory: %w", err)
	}
	return filepath.Join(home, directoryName, fileName), nil
}

// Load reads the config at path. A missing file isn't an error; the
// defaults are returned instead. Unset fields are filled with defaults.
func Load(path string) (*Config, error) {
	var config Config

	f, err := os.Open(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	if err == nil {
		defer f.Close()
		if err = json.NewDecoder(f).Decode(&config); err != nil {
			return nil, fmt.Errorf("decoding config from file: %w", err)
		}
	}

	if config.LoggedParams == nil {
		config.LoggedParams = make(map[string]logging.LoggedParam)
	}
	if config.LogDirectory == "" {
		config.LogDirectory = filepath.Join(filepath.Dir(path), "logs")
	}
	if config.LogFileNameFormat == "" {
		config.LogFileNameFormat = DefaultLogFileNameFormat
	}
	if config.LiveGraphMinutes <= 0 {
		config.LiveGraphMinutes = DefaultLiveGraphMinutes
	}
	return &config, nil
}

// Save writes config to path, creating the parent directory if needed.
func Save(path string, config *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err = os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}
