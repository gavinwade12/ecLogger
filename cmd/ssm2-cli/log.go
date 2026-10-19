package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

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

	logCmd.Flags().StringVar(&logFileFormat, "logFileFormat", "{{romID}}-{{timestamp}}.csv", "The format used for generating a log file name (path included). Variables can be injected using the format {{variableName}}. Supported variables: romID, timestamp.")
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

		conn, err := createSSM2Conn(port, ssm2Logger(cmd))
		if err != nil {
			return fmt.Errorf("creating new connection: %w", err)
		}
		defer conn.Close()

		ctx, cancel := context.WithCancel(context.Background())
		ctx, _ = signal.NotifyContext(ctx, os.Interrupt, os.Kill)
		defer cancel()

		// send an init command until a successful response or interruption is received
		stdOut := cmd.OutOrStdout()
		if !quiet {
			fmt.Fprintln(stdOut, "initializing with ECU and determining supported parameters...")
		}
		var ecu *ssm2.ECU
		for ecu == nil {
			ecu, err = conn.InitECU(ctx)
			if err == nil {
				break
			}

			if !errors.Is(err, ssm2.ErrReadTimeout) {
				return fmt.Errorf("sending init request: %w", err)
			}
			ecu = nil

			if err = conn.Close(); err != nil {
				return fmt.Errorf("closing ssm2 connection: %w", err)
			}

			conn, err = createSSM2Conn(port, ssm2Logger(cmd))
			if err != nil {
				return fmt.Errorf("creating new connection: %w", err)
			}
		}
		if !quiet {
			fmt.Fprintln(stdOut, "initialized")
		}

		logFileFormat = strings.NewReplacer(
			"{{romID}}", string(ecu.ROM_ID),
			"{{timestamp}}", time.Now().Format("20060102_150405"), //yyyyMMdd_hhmmss
		).Replace(logFileFormat)
		if !quiet {
			fmt.Fprintf(stdOut, "logging to file: %s\n", logFileFormat)
		}

		f, err := os.OpenFile(logFileFormat, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			return fmt.Errorf("opening file for logging: %w", err)
		}
		defer f.Close()

		// gather the parameters to log. start with read parameters, then do derived parameters
		loggedParams := []ssm2.Parameter{}
		loggedDerivedParams := []ssm2.DerivedParameter{}
		headers := []string{}
		addressesToRead := [][3]byte{}
		for _, cfgParam := range cfgParams {
			if cfgParam.Derived {
				continue
			}

			p := ssm2.Parameters[cfgParam.Id]
			headers = append(headers, fmt.Sprintf("%s (%s)", p.Name, cfgParam.Unit))
			loggedParams = append(loggedParams, p)
			addressesToRead = append(addressesToRead, p.Address.Address)
		}
		for _, cfgParam := range cfgParams {
			if !cfgParam.Derived {
				continue
			}

			dp := ssm2.DerivedParameters[cfgParam.Id]
			headers = append(headers, fmt.Sprintf("%s (%s)", dp.Name, cfgParam.Unit))
			loggedDerivedParams = append(loggedDerivedParams, dp)
		}

		if len(loggedParams) == 0 {
			return errors.New("no loggable parameters")
		}

		_, err = f.WriteString(strings.Join(headers, ",") + "\n")
		if err != nil {
			return fmt.Errorf("writing header line to log file: %w", err)
		}

		// begin reading the parameter values
		fmt.Fprintln(stdOut, "sending read addresses request")
		_, err = conn.SendReadAddressesRequest(ctx, addressesToRead, true)
		if err != nil {
			return fmt.Errorf("sending read address request: %w", err)
		}

		// read the packets in a go routine so we don't potentially block reads with our processing
		type packetResult struct {
			packet ssm2.Packet
			err    error
		}
		results := make(chan packetResult, 5) // buffer the channel in case it takes us longer to process than it takes to read

		go func() {
			for {
				p, err := conn.NextPacket(ctx)
				results <- packetResult{p, err}
			}
		}()

		// process the read packets until cancelled
		for {
			select {
			case result := <-results:
				if result.err != nil {
					if result.err == context.DeadlineExceeded {
						return nil
					}
					return fmt.Errorf("reading next packet from connection: %w", result.err)
				}

				data := result.packet.Data()
				index := 0
				for i, param := range loggedParams {
					pv := param.Value(data[index : index+param.Address.Length])
					val := strconv.FormatFloat(float64(pv.Value), 'f', 2, 32) + " " + string(pv.Unit)
					if i < len(loggedParams)-1 {
						val += ","
					}
					if _, err = f.WriteString(val); err != nil {
						return fmt.Errorf("writing parameter value: %w", err)
					}
					index += param.Address.Length
				}
			case <-ctx.Done():
				err := ctx.Err()
				if errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				return err
			}
		}
	},
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
