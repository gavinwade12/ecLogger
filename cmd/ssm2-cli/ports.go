package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gavinwade12/ecLogger/internal/serialport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	portsCmd.AddCommand(listPortsCmd)
	portsCmd.AddCommand(selectPortCmd)

	rootCmd.AddCommand(portsCmd)
}

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Manage the available ports",
}

var listPortsCmd = &cobra.Command{
	Use:   "list",
	Short: "List the available ports on the host",
	RunE: func(cmd *cobra.Command, args []string) error {
		ports, err := serialport.List()
		if err != nil {
			return err
		}

		listPorts(cmd.OutOrStdout(), ports)
		return nil
	},
}

func listPorts(w io.Writer, ports []serialport.Port) {
	for i, p := range ports {
		fmt.Fprintf(w, "[%d]:\tPortName: '%s'\n\tProduct: %s\n\tVID/PID: %s/%s\n\tUSB: %v\n\tSelected: %v\n",
			i, p.Name, p.Product, p.VendorID, p.ProductID, p.IsUSB, p.Name == port)
	}
}

var selectPortCmd = &cobra.Command{
	Use:          "set",
	Short:        "Set the port to use in the config file",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ports, err := serialport.List()
		if err != nil {
			return err
		}
		listPorts(cmd.OutOrStdout(), ports)
		fmt.Fprint(cmd.OutOrStdout(), "Port (index): ")

		input, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil && !(errors.Is(err, io.EOF) && input != "") {
			return err
		}

		i, err := parseSelection(input, len(ports))
		if err != nil {
			return err
		}

		portName := ports[i].Name
		viper.Set(portSettingName, portName)
		fmt.Fprintf(cmd.OutOrStdout(), "Selected '%s'\n", portName)

		return viper.WriteConfig()
	},
}

// parseSelection parses input as an index into a list of n ports.
func parseSelection(input string, n int) (int, error) {
	i, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, fmt.Errorf("parsing input as integer: %w", err)
	}
	if i < 0 || i >= n {
		return 0, errors.New("invalid selection")
	}
	return i, nil
}
