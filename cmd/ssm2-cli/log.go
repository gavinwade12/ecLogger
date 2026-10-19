package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/internal/serialport"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logFileFormat string

func init() {
	addLoggedParamCmd.Flags().StringVar(&paramID, "paramID", "", "The parameter Id to add")
	addLoggedParamCmd.Flags().StringVar(&unit, "unit", "", "The desired unit for the parameter")
	logCmd.AddCommand(addLoggedParamCmd)

	rootCmd.AddCommand(logCmd)

	logCmd.Flags().StringVar(&logFileFormat, "logFileFormat", "{{romId}}-{{timestamp}}.csv", "The format used for generating a log file name (path included). Variables can be injected using the format {{variableName}}. Supported variables: romId (hex), timestamp.")
}

type loggedParameter struct {
	Id      string     `mapstructure:"id"`
	Derived bool       `mapstructure:"derived"`
	Unit    units.Unit `mapstructure:"unit"`
}

var logCmd = &cobra.Command{
	Use:          "log",
	Short:        "Log the parameters with logging enabled that are also supprted by the connected ECU.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if port == "" {
			return errors.New("the port setting is required for logging")
		}
		if logFileFormat == "" {
			return errors.New("a log file name format is required")
		}

		var cfgParams []loggedParameter
		if err := viper.UnmarshalKey("logging.parameters", &cfgParams); err != nil {
			return fmt.Errorf("getting parameters configured for logging: %w", err)
		}
		if len(cfgParams) == 0 {
			return errors.New("no parameters are configured for logging")
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		conn, err := serialport.Open(port, ssm2Logger(cmd))
		if err != nil {
			return fmt.Errorf("creating new connection: %w", err)
		}
		defer func() { conn.Close() }() // conn is replaced when init times out

		// send an init command until a successful response or interruption is received
		stdOut := cmd.OutOrStdout()
		if !quiet {
			fmt.Fprintln(stdOut, "initializing with ECU and determining supported parameters...")
		}
		var ecu *ssm2.ECU
		for {
			ecu, err = conn.InitECU(ctx)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return nil // interrupted
			}
			if !errors.Is(err, ssm2.ErrReadTimeout) {
				return fmt.Errorf("sending init request: %w", err)
			}

			if err = conn.Close(); err != nil {
				return fmt.Errorf("closing ssm2 connection: %w", err)
			}
			conn, err = serialport.Open(port, ssm2Logger(cmd))
			if err != nil {
				return fmt.Errorf("creating new connection: %w", err)
			}
		}
		if !quiet {
			fmt.Fprintln(stdOut, "initialized")
		}

		params, derived, unitFor, err := selectParams(cfgParams, ecu)
		if err != nil {
			return err
		}

		fileName := logging.FileName(logFileFormat, ecu.ROM_ID, time.Now())
		if !quiet {
			fmt.Fprintf(stdOut, "logging to file: %s\n", fileName)
		}
		f, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("opening file for logging: %w", err)
		}
		defer f.Close()

		return logToCSV(ctx, conn, f, params, derived, unitFor, ssm2Logger(cmd))
	},
}

// selectParams returns the configured parameters that ecu supports, and the
// unit each should be logged in. Unknown and unsupported parameters are
// skipped, and an error is returned if none are left or a configured unit
// can't be converted to.
func selectParams(cfgParams []loggedParameter, ecu *ssm2.ECU) ([]ssm2.Parameter, []ssm2.DerivedParameter,
	map[string]units.Unit, error) {
	supported := map[string]bool{}
	for _, p := range ecu.SupportedParameters {
		supported[p.Id] = true
	}
	for _, p := range ecu.SupportedDerivedParameters {
		supported[p.Id] = true
	}

	var (
		params  []ssm2.Parameter
		derived []ssm2.DerivedParameter
		unitFor = map[string]units.Unit{}
	)
	for _, cfgParam := range cfgParams {
		if !supported[cfgParam.Id] {
			continue
		}

		var defaultUnit units.Unit
		if cfgParam.Derived {
			p, ok := ssm2.DerivedParameters[cfgParam.Id]
			if !ok {
				continue
			}
			derived = append(derived, p)
			defaultUnit = p.DefaultUnit
		} else {
			p, ok := ssm2.Parameters[cfgParam.Id]
			if !ok {
				continue
			}
			params = append(params, p)
			defaultUnit = p.DefaultUnit
		}

		u := cfgParam.Unit
		if u == "" {
			u = defaultUnit
		}
		if u != defaultUnit && !slices.Contains(units.ConvertibleTo(defaultUnit), u) {
			return nil, nil, nil, fmt.Errorf("%s can't be logged in %s", cfgParam.Id, u)
		}
		unitFor[cfgParam.Id] = u
	}

	if len(params) == 0 && len(derived) == 0 {
		return nil, nil, nil, errors.New("none of the configured parameters are supported by the ECU")
	}
	return params, derived, unitFor, nil
}

// logToCSV logs params and derived from conn to w as CSV until ctx is
// canceled or writing fails. Values are converted to the units in unitFor.
func logToCSV(ctx context.Context, conn ssm2.Connection, w io.Writer, params []ssm2.Parameter,
	derived []ssm2.DerivedParameter, unitFor map[string]units.Unit, l ssm2.Logger) error {
	csvWriter, err := logging.NewCSVWriter(w, params, derived, func(id string, _ units.Unit) units.Unit {
		return unitFor[id]
	})
	if err != nil {
		return fmt.Errorf("writing header line to log file: %w", err)
	}

	runner := logging.NewRunner(func(id string) (units.Unit, bool) {
		u, ok := unitFor[id]
		return u, ok
	}, l)

	writeErr := make(chan error, 1)
	runner.Subscribe("csv", func(values map[string]ssm2.ParameterValue) {
		if err := csvWriter.Write(time.Now(), values); err != nil {
			select {
			case writeErr <- err:
			default:
			}
		}
	})
	runner.Start(conn, params, derived)
	defer runner.Stop()

	select {
	case <-ctx.Done():
		return nil
	case err := <-writeErr:
		return fmt.Errorf("writing parameter values: %w", err)
	}
}

var paramID string
var unit string

var addLoggedParamCmd = &cobra.Command{
	Use:   "add_param",
	Short: "Adds a parameter to the logging config",
	RunE: func(cmd *cobra.Command, args []string) error {
		if paramID == "" {
			return errors.New("no paramID set")
		}
		if unit == "" {
			return errors.New("no unit set")
		}

		var cfgParams []loggedParameter
		if err := viper.UnmarshalKey("logging.parameters", &cfgParams); err != nil {
			return fmt.Errorf("getting parameters configured for logging: %w", err)
		}
		for _, p := range cfgParams {
			if p.Id == paramID {
				return errors.New("the parameter is already configured for logging")
			}
		}

		var derived bool
		_, ok := ssm2.Parameters[paramID]
		if !ok {
			_, ok := ssm2.DerivedParameters[paramID]
			if !ok {
				return errors.New("invalid paramID")
			}
			derived = true
		}

		cfgParams = append(cfgParams, loggedParameter{
			Id:      paramID,
			Derived: derived,
			Unit:    units.Unit(unit),
		})

		viper.Set("logging.parameters", cfgParams)
		return viper.WriteConfig()
	},
}
