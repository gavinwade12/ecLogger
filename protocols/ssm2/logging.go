package ssm2

import (
	"context"

	"github.com/pkg/errors"
)

// LoggingSession sends a continuous ReadAddressesRequest for the given parameters
// and then reads the response packets until the context is canceled. The results
// are sent on the returned channel, and the channel is closed when the context
// is canceled or too many consecutive errors are encountered during processing.
func LoggingSession(ctx context.Context, conn Connection, params []Parameter,
	derived []DerivedParameter) (<-chan map[string]ParameterValue, error) {
	addressesToRead := [][3]byte{}
	for _, param := range params {
		for i := 0; i < param.Address.Length; i++ {
			addressesToRead = append(addressesToRead, param.Address.Add(i))
		}
	}

	_, err := conn.SendReadAddressesRequest(ctx, addressesToRead, true)
	if err != nil {
		return nil, errors.Wrap(err, "sending read addresses request")
	}

	results := make(chan map[string]ParameterValue, 10)
	go processPackets(ctx, results, conn, params, derived)
	return results, nil
}

func processPackets(ctx context.Context, results chan<- map[string]ParameterValue,
	conn Connection, params []Parameter, derived []DerivedParameter) {
	errCount := 0
	for {
		select {
		case <-ctx.Done():
			close(results)
			return
		default:
			packet, err := conn.NextPacket(ctx)
			if err != nil {
				conn.logger().Debug(err.Error())
				errCount++
				if errCount == 3 {
					close(results)
					return
				}
				continue
			}
			errCount = 0

			data := packet.Data()
			addrIndex := 0
			values := make(map[string]ParameterValue)
			for _, param := range params {
				values[param.Id] = param.Value(data[addrIndex : addrIndex+param.Address.Length])
				addrIndex += param.Address.Length
			}
			for _, param := range derived {
				val, err := param.Value(values)
				if err != nil {
					conn.logger().Debugf("getting value from %s: %v\n", param.Id, err)
					continue
				}

				values[param.Id] = *val
			}

			results <- values
		}
	}
}
