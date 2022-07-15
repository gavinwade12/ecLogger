package main

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gavinwade12/ssm2/protocols/ssm2"
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

		derivedParamWithUnitExpr := regexp.MustCompile(`\[(P\d+):(.+?)\]`)
		derivedParamWithoutUnitExpr := regexp.MustCompile(`(^|[^"])(P\d+)`)

		parameterUnits := make(map[string]ssm2.Unit)
		params := strings.Builder{}
		derivedParams := strings.Builder{}
		for _, param := range ssmProtocol.Parameters {
			if len(param.Conversions) == 0 {
				return fmt.Errorf("no conversions for %s", param.Id)
			}

			var smallestConversion romRaiderParameterConversion
			for _, c := range param.Conversions {
				if smallestConversion.Expr == "" || len(c.Expr) < len(smallestConversion.Expr) {
					smallestConversion = c
				}
			}

			unit := ssm2.Unit(smallestConversion.Units)
			parameterUnits[param.Id] = unit
			unitString, err := getUnitConstName(unit)
			if err != nil {
				return err
			}

			if len(param.Depends) > 0 {
				derivedParams.WriteString(fmt.Sprintf(`
			"%s": {
				Id: "%s",
				Name: "%s",
				Description: "%s",
				`, param.Id, param.Id, param.Name, param.Description))

				depends := make([]string, len(param.Depends))
				for i, d := range param.Depends {
					depends[i] = `"` + d.Parameter + `"`
				}

				expr := smallestConversion.Expr
				for _, match := range derivedParamWithUnitExpr.FindAllStringSubmatch(expr, -1) {
					uName, _ := getUnitConstName(parameterUnits[match[1]])
					newExpr := fmt.Sprintf(`(params["%s"].SafeConvertTo(%s).Value)`, match[1], uName)
					expr = strings.Replace(expr, match[0], newExpr, 1)
				}
				for _, match := range derivedParamWithoutUnitExpr.FindAllString(expr, -1) {
					if match[0] != 'P' {
						match = match[1:]
					}
					newExpr := fmt.Sprintf(`(params["%s"].Value)`, match)
					expr = strings.Replace(expr, match, newExpr, 1)
				}

				derivedParams.WriteString(fmt.Sprintf(`DependsOnParameters: []string{%s},
				Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
					return &ParameterValue{%s, %s}, nil
				},
			},`, strings.Join(depends, ", "), expr, unitString))
			} else {
				params.WriteString(fmt.Sprintf(`
			"%s": {
				Id: "%s",
				Name: "%s",
				Description: "%s",
				CapabilityByteIndex: %d,
				CapabilityBitIndex: %d,
				`, param.Id, param.Id, param.Name, param.Description, param.EcuByteIndex, param.EcuBit))

				addressBytes, err := param.Address.GetAddressBytes()
				if err != nil {
					return errors.Wrap(err, "decoding parameter address bytes")
				}
				address := &ssm2.ParameterAddress{
					Address: addressBytes,
				}
				if param.Address.Length != nil {
					address.Length = *param.Address.Length
				} else {
					address.Length = 1
				}

				params.WriteString(fmt.Sprintf(`Address: &ParameterAddress{
					Address: [3]byte{0x%x, 0x%x, 0x%x},
					Length:  %d,
				},
				`, address.Address[0], address.Address[1], address.Address[2], address.Length))

				var expression string
				if param.Address.Length != nil && *param.Address.Length == 2 {
					expression = strings.ReplaceAll(smallestConversion.Expr, "x", "float32(binary.BigEndian.Uint32(v))")
				} else {
					expression = strings.ReplaceAll(smallestConversion.Expr, "x", "float32(v[0])")
				}
				params.WriteString(fmt.Sprintf(`Value: func(v []byte) ParameterValue {
					return ParameterValue{%s, %s}
				},
			},`, expression, unitString))
			}
		}

		err = os.WriteFile("ssm2_params_converted.go.gen", []byte(params.String()), os.ModePerm)
		if err != nil {
			return errors.Wrap(err, "writing converted params file")
		}

		err = os.WriteFile("ssm2_derived_params_converted.go.gen", []byte(derivedParams.String()), os.ModePerm)
		if err != nil {
			return errors.Wrap(err, "writing converted derived params file")
		}

		return nil
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
	return []byte{}, fmt.Errorf("dtc tmp address malformed %s", d.TmpAddr)
}

func (d romRaiderDTC) GetMemAddressBytes() ([]byte, error) {
	if len(d.MemAddr) > 2 {
		return hex.DecodeString(d.MemAddr[2:])
	}
	return []byte{}, fmt.Errorf("dtc address malformed %s", d.MemAddr)
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
	Depends      []romRaiderParameterDependency `xml:"depends>ref"`
}

type romRaiderParameterDependency struct {
	Parameter string `xml:"parameter,attr"`
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
	return [3]byte{}, fmt.Errorf("parameter address malformed '%s'", a.Address)
}

type romRaiderParameterConversion struct {
	Units     string  `xml:"units,attr"`
	Expr      string  `xml:"expr,attr"`
	Format    string  `xml:"format,attr"`
	GaugeMin  float64 `xml:"gauge_min,attr"`
	GaugeMax  float64 `xml:"gauge_max,attr"`
	GaugeStep float64 `xml:"gauge_step,attr"`
}

func getUnitConstName(u ssm2.Unit) (string, error) {
	var unitString string
	switch u {
	case ssm2.UnitMPH:
		unitString = "UnitMPH"
	case ssm2.UnitKMH:
		unitString = "UnitKMH"
	case ssm2.UnitMiles:
		unitString = "UnitMiles"
	case ssm2.UnitKilometers:
		unitString = "UnitKilometers"
	case ssm2.UnitRPM:
		unitString = "UnitRPM"
	case ssm2.UnitDegress:
		unitString = "UnitDegress"
	case ssm2.UnitF:
		unitString = "UnitF"
	case ssm2.UnitC:
		unitString = "UnitC"
	case ssm2.UnitPSI:
		unitString = "UnitPSI"
	case ssm2.UnitBAR:
		unitString = "UnitBAR"
	case ssm2.UnitKPA:
		unitString = "UnitKPA"
	case ssm2.UnitHPA:
		unitString = "UnitHPA"
	case ssm2.UnitMPA:
		unitString = "UnitMPA"
	case ssm2.UnitInHG:
		unitString = "UnitInHG"
	case ssm2.UnitMmHG:
		unitString = "UnitMmHG"
	case ssm2.UnitGS:
		unitString = "UnitGS"
	case ssm2.UnitAFR:
		unitString = "UnitAFR"
	case ssm2.UnitLambda:
		unitString = "UnitLambda"
	case ssm2.UnitVolts:
		unitString = "UnitVolts"
	case ssm2.UnitAmps:
		unitString = "UnitAmps"
	case ssm2.UnitMilliamps:
		unitString = "UnitMilliamps"
	case ssm2.UnitOhms:
		unitString = "UnitOhms"
	case ssm2.UnitMS:
		unitString = "UnitMS"
	case ssm2.UnitUS:
		unitString = "UnitUS"
	case ssm2.UnitPercent:
		unitString = "UnitPercent"
	case ssm2.UnitSteps:
		unitString = "UnitSteps"
	case ssm2.UnitGear:
		unitString = "UnitGear"
	case ssm2.UnitCount:
		unitString = "UnitCount"
	case ssm2.UnitMisfireCount:
		unitString = "UnitMisfireCount"
	case ssm2.UnitMultiplier:
		unitString = "UnitMultiplier"
	case ssm2.UnitIndex:
		unitString = "UnitIndex"
	case ssm2.UnitRaw:
		unitString = "UnitRaw"
	case ssm2.UnitDegreesCrankAngle:
		unitString = "UnitDegreesCrankAngle"
	case ssm2.UnitMM3PerStroke:
		unitString = "UnitMM3PerStroke"
	case ssm2.UnitMGPerCylinder:
		unitString = "UnitMGPerCylinder"
	case ssm2.UnitDegreesPerSecond:
		unitString = "UnitDegreesPerSecond"
	case ssm2.UnitMetersPerSecondSquared:
		unitString = "UnitMetersPerSecondSquared"
	case ssm2.UnitTime:
		unitString = "UnitTime"
	case ssm2.UnitGramsPerRev:
		unitString = "UnitGramsPerRev"
	case ssm2.UnitMPGUS:
		unitString = "UnitMPGUS"
	case ssm2.UnitMPGUK:
		unitString = "UnitMPGUK"
	case ssm2.UnitKMPerL:
		unitString = "UnitKMPerL"
	case ssm2.UnitLPer100K:
		unitString = "UnitLPer100K"
	case ssm2.UnitTimes:
		unitString = "UnitTimes"
	case ssm2.UnitGrams:
		unitString = "UnitGrams"
	case ssm2.UnitCoefficient:
		unitString = "UnitCoefficient"
	case ssm2.UnitNm:
		unitString = "UnitNm"
	default:
		return "", fmt.Errorf("unknown const for unit '%s'", u)
	}

	return unitString, nil
}
