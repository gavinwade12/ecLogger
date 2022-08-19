package ssm2_test

import (
	"context"
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
