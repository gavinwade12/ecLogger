package ssm2_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

func TestLoggingSession(t *testing.T) {
	params := []ssm2.Parameter{
		ssm2.Parameters["P7"],
		ssm2.Parameters["P8"],
		ssm2.Parameters["P12"],
		ssm2.Parameters["P21"],
		ssm2.Parameters["P160"],
	}
	derivedParams := []ssm2.DerivedParameter{
		ssm2.DerivedParameters["P200"],
		ssm2.DerivedParameters["P201"],
		ssm2.DerivedParameters["P237"],
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := ssm2.LoggingSession(ctx, ssm2.NewFakeConnection(time.Millisecond), params, derivedParams)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		values := <-session
		if len(values) != len(params)+len(derivedParams) {
			t.Fatalf("not all values are present")
		}
	}
}

func TestDecodeValues(t *testing.T) {
	rpm := ssm2.Parameters["P8"]  // 2 bytes
	maf := ssm2.Parameters["P12"] // 2 bytes
	iat := ssm2.Parameters["P11"] // 1 byte
	map_ := ssm2.Parameters["P7"] // 1 byte
	params := []ssm2.Parameter{rpm, maf, iat, map_}
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	t.Run("DecodesInOrder", func(t *testing.T) {
		data := []byte{0x1f, 0x40, 0x00, 0x64, 0x50, 0x20}
		values, err := ssm2.DecodeValues(data, params, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range params {
			want := p.Value(map[string][]byte{
				"P8": data[0:2], "P12": data[2:4], "P11": data[4:5], "P7": data[5:6],
			}[p.Id])
			if values[p.Id] != want {
				t.Errorf("%s = %v, want %v", p.Id, values[p.Id], want)
			}
		}
	})

	t.Run("ChecksDataLength", func(t *testing.T) {
		_, err := ssm2.DecodeValues(data[:5], params, nil, nil)
		if !errors.Is(err, ssm2.ErrInvalidResponseLength) {
			t.Fatalf("want ErrInvalidResponseLength. got: %v.", err)
		}
	})

	t.Run("DerivedDependsOnDerived", func(t *testing.T) {
		// P242 depends on P200, so it must be calculated after it regardless of order
		derived := []ssm2.DerivedParameter{
			ssm2.DerivedParameters["P242"],
			ssm2.DerivedParameters["P200"],
		}
		data := []byte{0x1f, 0x40, 0x00, 0x64, 0x50, 0x20}
		values, err := ssm2.DecodeValues(data, params, derived, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := values["P200"]; !ok {
			t.Fatal("P200 missing")
		}
		if v, ok := values["P242"]; !ok || v.Value == 0 {
			t.Fatalf("P242 = %v, %v; want a non-zero value", v, ok)
		}
	})

	t.Run("SkipsDerivedWithMissingDependencies", func(t *testing.T) {
		derived := []ssm2.DerivedParameter{ssm2.DerivedParameters["P201"]} // needs P21
		values, err := ssm2.DecodeValues(data, params, derived, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := values["P201"]; ok {
			t.Fatal("P201 should have been skipped")
		}
	})
}

// failingConnection accepts a read addresses request and then fails every read.
type failingConnection struct {
	memoryConnection
	reads int
}

func (c *failingConnection) NextPacket(ctx context.Context) (ssm2.Packet, error) {
	c.reads++
	return nil, errors.New("boom")
}

func TestLoggingSessionStopsAfterConsecutiveErrors(t *testing.T) {
	conn := &failingConnection{memoryConnection: memoryConnection{ram: map[[3]byte]byte{}}}
	session, err := ssm2.LoggingSession(context.Background(), conn,
		[]ssm2.Parameter{ssm2.Parameters["P8"]}, nil)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case _, ok := <-session:
		if ok {
			t.Fatal("expected no values")
		}
	case <-time.After(time.Second):
		t.Fatal("session didn't close")
	}
	if conn.reads != 3 {
		t.Fatalf("got %d reads, want 3", conn.reads)
	}
}

func TestLoggingSessionClosesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	session, err := ssm2.LoggingSession(ctx, ssm2.NewFakeConnection(time.Millisecond),
		[]ssm2.Parameter{ssm2.Parameters["P8"]}, nil)
	if err != nil {
		t.Fatal(err)
	}

	<-session
	cancel()

	// the channel must be closed even if nobody is reading the buffered values
	timeout := time.After(time.Second)
	for {
		select {
		case _, ok := <-session:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("session didn't close after cancel")
		}
	}
}

func TestFakeConnectionNextPacketRespectsContext(t *testing.T) {
	conn := ssm2.NewFakeConnection(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := conn.NextPacket(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled. got: %v.", err)
	}
}
