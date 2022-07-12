package ssm2

import (
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
)

// Parameter represents a parameter that can be read from the ECU.
type Parameter struct {
	Name        string
	Description string

	// CapabilityByteIndex points to the capability byte containing the parameter's flag.
	CapabilityByteIndex uint
	// CapabilityBitIndex is the index of the bit flag within the byte containing the parameter's flag.
	CapabilityBitIndex uint

	// Address is present when the parameter value is read from RAM instead of calculated
	Address *ParameterAddress

	Log bool
}

// ParameterAddress describes the address(es) containing the value for the parameter
// with an optional bit for switch parameters.
type ParameterAddress struct {
	Address [3]byte
	Length  int   // used when the value takes more than 1 address e.g. a 32-bit value on a 16-bit ECU
	Bit     *uint // used for switches
}

// LoadParametersFromFile loads a list of parameters from the given file.
func LoadParametersFromFile(file string) ([]Parameter, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	params := []Parameter{}
	if strings.ToLower(path.Ext(file)) != ".json" {
		return nil, errors.New("unknown file type (supported: json)")
	}

	if err = json.NewDecoder(f).Decode(&params); err != nil {
		return nil, errors.Wrap(err, "decoding parameters from json")
	}
	return params, nil
}

// SaveParametersToFile saves the parameters to the given file.
func SaveParametersToFile(params []Parameter, file string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	switch strings.ToLower(path.Ext(file)) {
	case ".json":
		return json.NewEncoder(f).Encode(params)
	default:
		return errors.New("unknown file type (supported: json)")
	}
}
