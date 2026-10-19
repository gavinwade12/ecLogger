package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const portSettingName string = "port"

var configFile string
var parameterFile string
var port string
var quiet bool
var verbose bool

func init() {
	cobra.OnInitialize(func() {
		initConfig()
		postInitCommands(rootCmd.Commands())
	})

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.ssm2.yaml)")
	rootCmd.PersistentFlags().StringVar(&port, portSettingName, "", "serial port to connect to. Example: /dev/ttyUSB0")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "quiet all log output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "provide verbose output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:           "ssm2-cli",
	Short:         "A CLI for interfacing with a Subaru ECU using the SSM2 protocol.",
	SilenceErrors: true,
}

func initConfig() {
	home := ""
	if configFile == "" {
		var err error
		if home, err = os.UserHomeDir(); err != nil {
			log.Fatalf("finding home directory: %v\n", err)
		}
	}

	viper.AutomaticEnv()

	if err := loadConfig(viper.GetViper(), configFile, home); err != nil {
		log.Fatal(err)
	}
}

// loadConfig reads configFile, or .ssm2.yaml in home when configFile is
// empty, and creates the file when it doesn't exist.
func loadConfig(v *viper.Viper, configFile, home string) error {
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.AddConfigPath(home)
		v.SetConfigName(".ssm2")
		v.SetConfigType("yaml")
	}

	err := v.ReadInConfig()
	if err == nil {
		return nil
	}
	if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading config file: %w", err)
	}

	if configFile != "" {
		err = v.SafeWriteConfigAs(configFile)
	} else {
		err = v.SafeWriteConfig()
	}
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	return nil
}

func postInitCommands(commands []*cobra.Command) {
	for _, cmd := range commands {
		presetRequiredFlags(cmd)
		if cmd.HasSubCommands() {
			postInitCommands(cmd.Commands())
		}
	}
}

func presetRequiredFlags(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			cmd.Flags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}

func ssm2Logger(cmd *cobra.Command) ssm2.Logger {
	if !verbose {
		return ssm2.NopLogger
	}
	return ssm2.DefaultLogger(cmd.OutOrStdout())
}
