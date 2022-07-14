package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gavinwade12/ssm2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	portsCmd.AddCommand(listPortsCmd)
	portsCmd.AddCommand(selectPortCmd)
	portsCmd.PersistentFlags().IntVar(&byteCount, "byteCount", 0, "The amount of bytes to read from the port")
	portsCmd.PersistentFlags().BoolVar(&readPacket, "readPacket", false, "Read an entire packet instead of the specified byte count")

	writeToPortCmd.Flags().StringVar(&bytes, "bytes", "", "The bytes to send over the port")
	writeToPortCmd.Flags().BoolVar(&readBytes, "readBytes", false, "Read a byteCount after writing the bytes")
	writeToPortCmd.Flags().IntVar(&pauseMS, "pauseMS", 0, "The number of seconds to pause after writing before reading")
	portsCmd.AddCommand(writeToPortCmd)

	portsCmd.AddCommand(readFromPortCmd)

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
		fmt.Fprintf(w, "[%d]:\tPortName: '%s'\n\tProduct: %s\n\tVID/PID: %s/%s\n\tUSB: %v\n\tSelected: %v\n",
			i, p.PortName, p.Product, p.VendorID, p.ProductID, p.IsUSB, p.PortName == port)
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
var readBytes bool
var pauseMS int

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

		sp := conn.SerialPort()
		n, err := sp.Write(b)
		if err != nil {
			conn.Close()
			return errors.Wrap(err, "writing to serial port")
		}
		if n != len(b) {
			conn.Close()
			return errors.New(fmt.Sprintf("wrote %d bytes but expected to write %d bytes", n, len(b)))
		}

		if pauseMS > 0 {
			dur := time.Millisecond * time.Duration(pauseMS)
			fmt.Printf("pausing for %s\n", dur)
			<-time.NewTimer(dur).C
		}

		if readBytes {
			if err = readBytesFromConn(cmd.Context(), conn); err != nil {
				return errors.Wrap(err, "reading bytes")
			}
		}

		if err = conn.Close(); err != nil {
			return errors.Wrap(err, "closing ssm2 connection")
		}

		return nil
	},
}

var byteCount int
var readPacket bool

var readFromPortCmd = &cobra.Command{
	Use:          "read",
	Short:        "Read bytes from the selected port",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if port == "" {
			return errors.New("a port is required for reading")
		}
		conn, err := ssm2.NewConnection(port, ssm2Logger(cmd))
		if err != nil {
			return errors.Wrap(err, "creating new ssm2 connection")
		}

		return readBytesFromConn(cmd.Context(), conn)
	},
}

func readBytesFromConn(ctx context.Context, conn *ssm2.Connection) error {
	if readPacket {
		b, err := conn.NextPacket(ctx)
		if err != nil {
			return errors.Wrap(err, "reading packet")
		}

		fmt.Print("read bytes: ")
		for _, bb := range b {
			fmt.Printf("0x%x ", bb)
		}
		fmt.Println()
		return nil
	}

	if byteCount < 1 {
		return errors.New("byteCount must be > 0")
	}

	var (
		b = make([]byte, byteCount)
		c int
	)
	for c < byteCount {
		cc, err := conn.SerialPort().Read(b[c:])
		if err != nil {
			return errors.Wrap(err, "reading chunk")
		}

		fmt.Print("read byte chunk: ")
		for _, bb := range b[c : c+cc] {
			fmt.Printf("0x%x ", bb)
		}
		fmt.Println()
		c += cc
	}

	fmt.Print("read bytes: ")
	for _, bb := range b {
		fmt.Printf("0x%x ", bb)
	}
	fmt.Println()

	return nil
}
