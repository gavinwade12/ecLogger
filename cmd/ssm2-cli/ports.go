package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bug.st/serial/enumerator"
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
		ports, err := availablePorts()
		if err != nil {
			return err
		}

		listPorts(cmd.OutOrStdout(), ports)
		return nil
	},
}

func listPorts(w io.Writer, ports []serialPort) {
	for i, p := range ports {
		fmt.Fprintf(w, "[%d]:\tPortName: '%s'\n\tProduct: %s\n\tVID/PID: %s/%s\n\tUSB: %v\n\tSelected: %v\n",
			i, p.PortName, p.Product, p.VendorID, p.ProductID, p.IsUSB, p.PortName == port)
	}
}

var selectPortCmd = &cobra.Command{
	Use:          "set",
	Short:        "Set the port to use in the config file",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ports, err := availablePorts()
		if err != nil {
			return err
		}
		listPorts(cmd.OutOrStdout(), ports)
		fmt.Fprint(cmd.OutOrStdout(), "Port (index): ")

		input, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil {
			return err
		}

		// trim the newline from the input before parsing
		input = input[:len(input)-1]
		if input[len(input)-1] == '\r' {
			input = input[:len(input)-1]
		}

		i, err := strconv.Atoi(input)
		if err != nil {
			return errors.Wrap(err, "parsing input as integer")
		}

		if i < 0 || i >= len(ports) {
			return errors.New("invalid selection")
		}

		portName := ports[i].PortName
		viper.Set(portSettingName, portName)
		fmt.Fprintf(cmd.OutOrStdout(), "Selected '%s'\n", portName)

		return viper.WriteConfig()
	},
}

type serialPort struct {
	PortName  string
	Product   string
	IsUSB     bool
	VendorID  string
	ProductID string
}

// availablePorts returns all available serial ports on the current host.
func availablePorts() ([]serialPort, error) {
	list, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}

	ports := make([]serialPort, len(list))
	for i, p := range list {
		ports[i] = serialPort{
			PortName:  p.Name,
			Product:   p.Product,
			IsUSB:     p.IsUSB,
			VendorID:  p.VID,
			ProductID: p.PID,
		}
	}

	return ports, nil
}
