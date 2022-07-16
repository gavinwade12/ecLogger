package main

import (
	"log"
	"os"
	"path"

	"github.com/gavinwade12/ssm2/protocols/ssm2"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
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

func init() {
	cobra.OnInitialize(func() {
		initConfig()
		postInitCommands(rootCmd.Commands())
	})

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.ssm2.yaml)")
	rootCmd.PersistentFlags().StringVar(&parameterFile, "parameters", "", "parameter file (default is $HOME/.ssm.parameters.json)")
	rootCmd.PersistentFlags().StringVar(&port, portSettingName, "", "The serial port to connect to. Example: /dev/ttyUSB0")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Quiet log output")
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if parameterFile == "" {
			home, err := homedir.Dir()
			if err != nil {
				return errors.Wrap(err, "finding home directory for parameter file")
			}

			parameterFile = path.Join(home, ".ssm2.parameters.json")
		}

		return nil
	},
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(path.Base(configFile))
		viper.AddConfigPath(path.Dir(configFile))
	} else {
		home, err := homedir.Dir()
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
	if quiet {
		return ssm2.NopLogger
	}
	return ssm2.DefaultLogger(cmd.OutOrStdout())
}

func createSSM2Conn(port string, l ssm2.Logger) (*ssm2.Connection, error) {
	l.Debugf("opening serial connection to port %s", port)
	sp, err := serial.Open(port, &serial.Mode{
		BaudRate: ssm2.ConnectionBaudRate,
		DataBits: ssm2.ConnectionDataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "opening serial port '%s'", port)
	}

	if err = sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
		return nil, errors.Wrap(err, "setting connection read timeout")
	}

	return ssm2.NewConnection(sp, l), nil
}
