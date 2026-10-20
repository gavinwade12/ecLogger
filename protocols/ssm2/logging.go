package ssm2

import (
	"context"
	"fmt"
)

// maxConsecutiveLoggingErrors is the number of consecutive packet errors
// tolerated before a logging session gives up.
const maxConsecutiveLoggingErrors = 3

// LoggingSession sends a continuous ReadAddressesRequest for the given parameters
// and then reads the response packets until the context is canceled. The results
// are sent on the returned channel, and the channel is closed when the context
// is canceled or too many consecutive errors are encountered during processing.
func LoggingSession(ctx context.Context, conn Connection, params []Parameter,
	derived []DerivedParameter) (<-chan map[string]ParameterValue, error) {
	_, err := conn.SendReadAddressesRequest(ctx, addressesToRead(params), true)
	if err != nil {
		return nil, fmt.Errorf("sending read addresses request: %w", err)
	}

	results := make(chan map[string]ParameterValue, 10)
	go processPackets(ctx, results, conn, params, derived)
	return results, nil
}

// addressesToRead returns every address that must be read to get the values for params.
func addressesToRead(params []Parameter) [][3]byte {
	addresses := [][3]byte{}
	for _, param := range params {
		for i := 0; i < param.Address.Length; i++ {
			addresses = append(addresses, param.Address.Add(uint32(i)))
		}
	}
	return addresses
}

func processPackets(ctx context.Context, results chan<- map[string]ParameterValue,
	conn Connection, params []Parameter, derived []DerivedParameter) {
	defer close(results)
	log := loggerFor(conn)

	errCount := 0
	for ctx.Err() == nil {
		packet, err := conn.NextPacket(ctx)
		if err == nil {
			var values map[string]ParameterValue
			values, err = DecodeValues(packet.Data(), params, derived, log)
			if err == nil {
				errCount = 0
				select {
				case results <- values:
				case <-ctx.Done():
				}
				continue
			}
		}

		log.Debug(err.Error())
		errCount++
		if errCount == maxConsecutiveLoggingErrors {
			return
		}
	}
}

// DecodeValues decodes the data from a read addresses response into parameter
// values. data must contain the bytes for params in order, as requested by
// LoggingSession. Derived parameters are then calculated from the decoded values,
// in dependency order. Derived parameters whose dependencies are missing are skipped.
func DecodeValues(data []byte, params []Parameter, derived []DerivedParameter,
	log Logger) (map[string]ParameterValue, error) {
	if log == nil {
		log = NopLogger
	}

	want := 0
	for _, param := range params {
		want += param.Address.Length
	}
	if len(data) != want {
		return nil, fmt.Errorf("%w: want %d data bytes, got %d",
			ErrInvalidResponseLength, want, len(data))
	}

	values := make(map[string]ParameterValue, len(params)+len(derived))
	addrIndex := 0
	for _, param := range params {
		values[param.Id] = param.Value(data[addrIndex : addrIndex+param.Address.Length])
		addrIndex += param.Address.Length
	}

	// derived parameters may depend on other derived parameters, so keep
	// making passes until nothing else can be calculated
	pending := append([]DerivedParameter(nil), derived...)
	for progressed := true; progressed && len(pending) > 0; {
		progressed = false
		remaining := pending[:0]
		for _, param := range pending {
			if !dependenciesMet(param, values) {
				remaining = append(remaining, param)
				continue
			}
			progressed = true

			val, err := param.Value(values)
			if err != nil {
				log.Debugf("getting value from %s: %v\n", param.Id, err)
				continue
			}
			values[param.Id] = *val
		}
		pending = remaining
	}
	for _, param := range pending {
		log.Debugf("skipping %s: missing dependencies\n", param.Id)
	}

	return values, nil
}

func dependenciesMet(param DerivedParameter, values map[string]ParameterValue) bool {
	for _, dep := range param.DependsOnParameters {
		if _, ok := values[dep]; !ok {
			return false
		}
	}
	return true
}
