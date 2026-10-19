package serialport

import (
	"errors"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"go.bug.st/serial"
)

type fakePort struct {
	serial.Port
	timeoutErr, resetErr error

	timeout time.Duration
	reset   bool
}

func (p *fakePort) SetReadTimeout(t time.Duration) error {
	p.timeout = t
	return p.timeoutErr
}

func (p *fakePort) ResetInputBuffer() error {
	p.reset = true
	return p.resetErr
}

func TestSetUp(t *testing.T) {
	p := &fakePort{}
	if err := setUp(p); err != nil {
		t.Fatal(err)
	}
	if p.timeout != ssm2.ConnectionReadTimeout || !p.reset {
		t.Fatalf("timeout = %s, reset = %v; want %s, true", p.timeout, p.reset, ssm2.ConnectionReadTimeout)
	}

	if err := setUp(&fakePort{timeoutErr: errors.New("boom")}); err == nil {
		t.Error("expected the read timeout error")
	}
	if err := setUp(&fakePort{resetErr: errors.New("boom")}); err == nil {
		t.Error("expected the reset error")
	}
}
