package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gavinwade12/ssm2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	portsCmd.AddCommand(listPortsCmd)
	portsCmd.AddCommand(selectPortCmd)

	writeToPortCmd.Flags().StringVar(&bytes, "bytes", "", "The bytes to send over the port")
	portsCmd.AddCommand(writeToPortCmd)

	readFromPortCmd.Flags().IntVar(&byteCount, "byteCount", 0, "The amount of bytes to read from the port")
	portsCmd.AddCommand(readFromPortCmd)

	portsCmd.AddCommand(readPacketFromPortCmd)

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
		ports, err := ssm2.AvailablePorts()
		if err != nil {
			return err
		}

		listPorts(cmd.OutOrStdout(), ports)
		return nil
	},
}

func listPorts(w io.Writer, ports []ssm2.SerialPort) {
	for i, p := range ports {
		fmt.Fprintf(w, "[%d]:\tPortName: '%s'\n\tDescription: `%s`\n\tUSB: %v\n\tSelected: %v\n",
			i, p.PortName, p.Description, p.IsUSB, p.PortName == port)
	}
}

var selectPortCmd = &cobra.Command{
	Use:          "set",
	Short:        "Set the port to use in the config file",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ports, err := ssm2.AvailablePorts()
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

var bytes string

var writeToPortCmd = &cobra.Command{
	Use:   "write",
	Short: "Writes the given bytes to the selected port",
	RunE: func(cmd *cobra.Command, args []string) error {
		bytes = strings.NewReplacer(" ", "", "0x", "").Replace(bytes)
		if bytes == "" {
			return errors.New("no bytes supplied")
		}

		b, err := hex.DecodeString(bytes)
		if err != nil {
			return errors.Wrap(err, "decoding bytes")
		}

		fmt.Print("writing bytes: ")
		for _, bb := range b {
			fmt.Printf("0x%x ", bb)
		}
		fmt.Println()

		if port == "" {
			return errors.New("a port is required for sending")
		}
		conn, err := ssm2.NewConnection(port, ssm2Logger(cmd))
		if err != nil {
			return errors.Wrap(err, "creating new ssm2 connection")
		}
		defer conn.Close()

		n, err := conn.SerialPort().Write(b)
		if err != nil {
			conn.Close()
			return errors.Wrap(err, "writing to serial port")
		}
		if n != len(b) {
			conn.Close()
			return errors.New(fmt.Sprintf("wrote %d bytes but expected to write %d bytes", n, len(b)))
		}

		if err = conn.Close(); err != nil {
			return errors.Wrap(err, "closing ssm2 connection")
		}

		return nil
	},
}

var byteCount int

var readFromPortCmd = &cobra.Command{
	Use:          "read",
	Short:        "Read bytes from the selected port",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if byteCount < 1 {
			return errors.New("byte count must be at least 1")
		}

		if port == "" {
			return errors.New("a port is required for reading")
		}
		conn, err := ssm2.NewConnection(port, ssm2Logger(cmd))
		if err != nil {
			return errors.Wrap(err, "creating new ssm2 connection")
		}

		b := make([]byte, byteCount)
		_, err = conn.SerialPort().Read(b)
		if err != nil {
			conn.Close()
			return errors.Wrap(err, "reading from serial port")
		}

		fmt.Print("read bytes: ")
		for _, bb := range b {
			fmt.Printf("0x%x ", bb)
		}
		fmt.Println()

		return nil
	},
}

var readPacketFromPortCmd = &cobra.Command{
	Use:   "read_packet",
	Short: "Read an entire packet from the selected port",
	RunE: func(cmd *cobra.Command, args []string) error {
		if port == "" {
			return errors.New("a port is required for reading")
		}
		conn, err := ssm2.NewConnection(port, ssm2Logger(cmd))
		if err != nil {
			return errors.Wrap(err, "creating new ssm2 connection")
		}

		packet, err := conn.NextPacket(cmd.Context())
		if err != nil {
			conn.Close()
			return errors.Wrap(err, "reading packet")
		}

		fmt.Print("read bytes: ")
		for _, b := range packet {
			fmt.Printf("0x%x ", b)
		}
		fmt.Println()

		if err = conn.Close(); err != nil {
			return errors.Wrap(err, "closing ssm2 connection")
		}
		return nil
	},
}
