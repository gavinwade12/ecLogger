package main

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"

	"github.com/gavinwade12/ssm2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var logDefFile string

func init() {
	romRaiderLogDefCmd.Flags().StringVar(&logDefFile, "logDefFile", "", "Path to a RomRaider log definition file")

	convertCmd.AddCommand(romRaiderLogDefCmd)
	rootCmd.AddCommand(convertCmd)
}

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert different things",
}

var romRaiderLogDefCmd = &cobra.Command{
	Use:          "rrLogDef",
	Short:        "convert a RomRaider log definition file to a ssm2 parameter file",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if logDefFile == "" {
			return errors.New("a log definition file is required")
		}

		src, err := os.Open(logDefFile)
		if err != nil {
			return errors.Wrap(err, "opening RomRaider log definition file")
		}
		defer src.Close()

		rrLogger := romRaiderLogger{}
		if err = xml.NewDecoder(src).Decode(&rrLogger); err != nil {
			return errors.Wrap(err, "decoding RomRaider log definition file XML")
		}

		var ssmProtocol *romRaiderProtocol
		for _, protocol := range rrLogger.Protocols {
			if protocol.Id == "SSM" {
				ssmProtocol = &protocol
				break
			}
		}
		if ssmProtocol == nil {
			return errors.New("could not find SSM protocol in RomRaider log defintion file")
		}

		params := make([]ssm2.Parameter, len(ssmProtocol.Parameters))
		for i, param := range ssmProtocol.Parameters {
			var address *ssm2.ParameterAddress
			if param.Address.Address != "" {
				addressBytes, err := param.Address.GetAddressBytes()
				if err != nil {
					return errors.Wrap(err, "decoding parameter address bytes")
				}
				address = &ssm2.ParameterAddress{
					Address: addressBytes,
				}
				if param.Address.Length != nil {
					address.Length = *param.Address.Length
				} else {
					address.Length = 1
				}
				if param.Address.Bit != nil {
					b := uint(*param.Address.Bit)
					address.Bit = &b
				}
			}

			params[i] = ssm2.Parameter{
				Name:                param.Name,
				Description:         param.Description,
				CapabilityByteIndex: param.EcuByteIndex,
				CapabilityBitIndex:  param.EcuBit,
				Address:             address,
			}
		}

		return ssm2.SaveParametersToFile(params, parameterFile)
	},
}

type romRaiderLogger struct {
	Version   string              `xml:"version,attr"`
	Protocols []romRaiderProtocol `xml:"protocols>protocol"`
}

type romRaiderProtocol struct {
	Id             string               `xml:"id,attr"`
	Baud           int                  `xml:"baud,attr"`
	DataBits       int                  `xml:"databits,attr"`
	StopBits       int                  `xml:"stopbits,attr"`
	Parity         int                  `xml:"parity,attr"`
	ConnectTimeout int                  `xml:"connect_timeout,attr"`
	SendTimeout    int                  `xml:"send_timeout,attr"`
	Parameters     []romRaiderParameter `xml:"parameters>parameter"`
	Dtcs           []romRaiderDTC       `xml:"dtcodes>dtcode"`
}

type romRaiderDTC struct {
	Id          string `xml:"id,attr"`
	Name        string `xml:"name,attr"`
	Description string `xml:"desc,attr"`
	TmpAddr     string `xml:"tmpaddr,attr"`
	MemAddr     string `xml:"memaddr,attr"`
	Bit         uint   `xml:"bit,attr"`
	Set         bool
	Stored      bool
}

func (d romRaiderDTC) GetTmpAddressBytes() ([]byte, error) {
	if len(d.TmpAddr) > 2 {
		return hex.DecodeString(d.TmpAddr[2:])
	}
	return []byte{}, fmt.Errorf("Dtc Temp Address malformed %s", d.TmpAddr)
}

func (d romRaiderDTC) GetMemAddressBytes() ([]byte, error) {
	if len(d.MemAddr) > 2 {
		return hex.DecodeString(d.MemAddr[2:])
	}
	return []byte{}, fmt.Errorf("Dtc Address malformed %s", d.MemAddr)
}

// TODO: Some of these have dependencies on other params, and are actually
// derived values
type romRaiderParameter struct {
	Id           string                         `xml:"id,attr"`
	Name         string                         `xml:"name,attr"`
	Description  string                         `xml:"desc,attr"`
	EcuByteIndex uint                           `xml:"ecubyteindex,attr"`
	EcuBit       uint                           `xml:"ecubit,attr"`
	Target       uint                           `xml:"target,attr"`
	Address      romRaiderParameterAddress      `xml:"address"`
	Conversions  []romRaiderParameterConversion `xml:"conversions>conversion"`
}

type romRaiderParameterAddress struct {
	Address string `xml:",chardata"`
	Length  *int   `xml:"length,attr"`
	Bit     *int   `xml:"bit,attr"`
}

func (a romRaiderParameterAddress) GetAddressBytes() ([3]byte, error) {
	if len(a.Address) > 2 {
		b, err := hex.DecodeString(a.Address[2:])
		if err != nil {
			return [3]byte{}, err
		}

		if len(b) == 3 {
			return [3]byte{b[0], b[1], b[2]}, nil
		}
	}
	return [3]byte{}, fmt.Errorf("Parameter Address malformed '%s'", a.Address)
}

type romRaiderParameterConversion struct {
	Units     string  `xml:"units,attr"`
	Expr      string  `xml:"expr,attr"`
	Format    string  `xml:"format,attr"`
	GaugeMin  float64 `xml:"gauge_min,attr"`
	GaugeMax  float64 `xml:"gauge_max,attr"`
	GaugeStep float64 `xml:"gauge_step,attr"`
}
