package ssm2

import (
	"context"
	"math/rand"
	"time"
)

type fakeConnection struct {
	latency time.Duration
	ticker  *time.Ticker

	continuousAddressRead bool
	addresses             int
}

// NewFakeConnection returns a new Connection that
// isn't connected to a real device. It returns fake
// data on an interval based on the given latency.
func NewFakeConnection(latency time.Duration) Connection {
	return &fakeConnection{latency: latency}
}

// InitECU returns fake ECU data with all supported parameters.
func (c *fakeConnection) InitECU(ctx context.Context) (*ECU, error) {
	params := make([]Parameter, len(Parameters))
	i := 0
	for _, p := range Parameters {
		params[i] = p
		i++
	}
	i = 0
	derivedParams := make([]DerivedParameter, len(DerivedParameters))
	for _, p := range DerivedParameters {
		derivedParams[i] = p
		i++
	}

	return &ECU{
		SSM_ID:                     []byte{0x00, 0x00, 0x01},
		ROM_ID:                     []byte{0x00, 0x00, 0x00, 0x00, 0x01},
		SupportedParameters:        params,
		SupportedDerivedParameters: derivedParams,
	}, nil
}

// SendReadAddressesRequest returns an address response packet and
// optionally configures the connection to continue returning
// address response packets on each call to NextPacket().
func (c *fakeConnection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (Packet, error) {
	c.continuousAddressRead = continous
	c.addresses = len(addresses)
	c.ticker = time.NewTicker(c.latency)

	return c.addressResponsePacket(), nil
}

// NextPacket waits for the connection's latency and then returns
// a packet.
func (c *fakeConnection) NextPacket(ctx context.Context) (Packet, error) {
	<-c.ticker.C
	if c.continuousAddressRead {
		return c.addressResponsePacket(), nil
	}
	return Packet{}, nil
}

// Close does nothing.
func (c *fakeConnection) Close() error {
	return nil
}

func (c *fakeConnection) logger() Logger {
	return NopLogger
}

func (c *fakeConnection) addressResponsePacket() Packet {
	resp := make(Packet, PacketHeaderSize+c.addresses+1)
	resp[0] = PacketMagicByte
	resp[1] = DeviceDiagnosticTool
	resp[2] = DeviceEngine
	resp[3] = byte(c.addresses + 1)
	resp[4] = CommandReadAddressesResponse

	for i := 0; i < c.addresses; i++ {
		resp[i+5] = byte(rand.Intn(20) + 1)
	}
	resp[len(resp)-1] = CalculateChecksum(resp)

	return resp
}
