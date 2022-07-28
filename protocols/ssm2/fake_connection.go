package ssm2

import (
	"context"
	"math/rand"
	"time"
)

type fakeConnection struct {
	ticker *time.Ticker

	continuousAddressRead bool
	addresses             int
}

func NewFakeConnection(latency time.Duration) Connection {
	return &fakeConnection{ticker: time.NewTicker(latency)}
}

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

func (c *fakeConnection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (Packet, error) {
	c.continuousAddressRead = continous
	c.addresses = len(addresses)

	return c.addressResponsePacket(), nil
}

func (c *fakeConnection) NextPacket(ctx context.Context) (Packet, error) {
	<-c.ticker.C
	if c.continuousAddressRead {
		return c.addressResponsePacket(), nil
	}
	return Packet{}, nil
}

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
