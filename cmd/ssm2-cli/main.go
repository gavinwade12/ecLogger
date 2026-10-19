package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.bug.st/serial"
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
	if configFile != "" {
		viper.SetConfigFile(path.Base(configFile))
		viper.AddConfigPath(path.Dir(configFile))
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("finding home directory: %v\n", err)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".ssm2")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			if err = viper.SafeWriteConfig(); err != nil {
				log.Fatalf("creating config file: %v\n", err)
			}
		} else {
			log.Fatalf("reading config file: %v\n", err)
		}
	}
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

func createSSM2Conn(port string, l ssm2.Logger) (ssm2.Connection, error) {
	l.Debugf("opening serial port %s", port)
	sp, err := serial.Open(port, &serial.Mode{
		BaudRate: ssm2.ConnectionBaudRate,
		DataBits: ssm2.ConnectionDataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, fmt.Errorf("opening serial port '%s': %w", port, err)
	}

	if err = sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
		return nil, fmt.Errorf("setting serial port read timeout: %w", err)
	}
	if err = sp.ResetInputBuffer(); err != nil {
		return nil, fmt.Errorf("resetting input buffer: %w", err)
	}

	return ssm2.NewConnection(sp, l), nil
}
