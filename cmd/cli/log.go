package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/gavinwade12/ssm2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var logFileFormat string

func init() {
	rootCmd.AddCommand(logCmd)

	logCmd.Flags().StringVar(&logFileFormat, "logFileFormat", "{{romID}}-{{timestamp}}.csv", "The format used for generating a log file name (path included). Variables can be injected using the format {{variableName}}. Supported variables: romID, timestamp.")
}

var logCmd = &cobra.Command{
	Use:          "log",
	Short:        "Log the parameters with logging enabled that are also supprted by the connected ECU.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if port == "" {
			return errors.New("the port setting is required for logging")
		}

		conn, err := ssm2.NewConnection(port, ssm2Logger(cmd))
		if err != nil {
			return errors.Wrap(err, "creating new connection")
		}
		defer conn.Close()

		ctx, _ := signal.NotifyContext(context.Background(),
			os.Interrupt, os.Kill)

		// send an init command until a successful response or interruption is received
		fmt.Fprintln(cmd.OutOrStdout(), "initializing with ECU and determining supported parameters...")
		var resp ssm2.InitResponse
		for resp == nil {
			resp, err = conn.SendInitCommand(ctx)
			if err == nil {
				break
			}

			if !errors.Is(err, ssm2.ErrorReadTimeout) {
				return errors.Wrap(err, "sending init request")
			}
			resp = nil

			fmt.Println("read timed out")
			if err = conn.Initialize(); err != nil {
				return errors.Wrap(err, "re-initailizing connection")
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "initialized")

		// gather the parameters to log
		capabilities := resp.Capabilities()
		params, err := ssm2.LoadParametersFromFile(parameterFile)
		if err != nil {
			return errors.Wrap(err, "loading parameters from file")
		}

		loggedParams := []ssm2.Parameter{}
		addressesToRead := [][3]byte{}
		for _, param := range params {
			if !param.Log || !capabilities.Contains(param) {
				continue
			}

			loggedParams = append(loggedParams, param)
			if param.Address != nil {
				addressesToRead = append(addressesToRead, param.Address.Address)
			}
		}

		if len(loggedParams) == 0 {
			return errors.New("no loggable parameters")
		}

		// begin reading the parameter values
		fmt.Fprintln(cmd.OutOrStdout(), "sending read addresses request")
		_, err = conn.SendReadAddressesRequest(ctx, addressesToRead, true)
		if err != nil {
			return errors.Wrap(err, "sending read address request")
		}

		// read the packets in a go routine so we don't potentially block reads with our processing
		type packetResult struct {
			packet ssm2.Packet
			err    error
		}
		results := make(chan packetResult, 5) // buffer the channel in case it takes us longer to process than it takes to read

		go func() {
			p, err := conn.NextPacket(ctx)
			results <- packetResult{p, err}
		}()

		// process the read packets
		for result := range results {
			if result.err != nil {
				return errors.Wrap(err, "reading next packet from connection")
			}

			data := result.packet.Data()
			index := 0
			for _, param := range loggedParams {
				if param.Address == nil {
					continue
				}

				value := data[index : index+param.Address.Length]
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", param.Name, value)
			}
		}

		return nil
	},
}
