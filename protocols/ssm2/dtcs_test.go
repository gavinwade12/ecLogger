package ssm2_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

func dtcIDs(dtcs []ssm2.DTC) []string {
	ids := make([]string, len(dtcs))
	for i, d := range dtcs {
		ids[i] = d.ID
	}
	sort.Strings(ids)
	return ids
}

func TestReadDTCs(t *testing.T) {
	d3 := ssm2.DTCs["D3"]     // bit 2 of its byte
	d616 := ssm2.DTCs["D616"] // bit 7, in the highest addresses (last batch)
	d1 := ssm2.DTCs["D1"]     // bit 0, shares a byte with D3

	t.Run("RequestsEveryAddressOnce", func(t *testing.T) {
		conn := &memoryConnection{ram: map[[3]byte]byte{}}
		if _, err := ssm2.ReadSetDTCs(context.Background(), conn); err != nil {
			t.Fatal(err)
		}

		want := map[[3]byte]bool{}
		for _, d := range ssm2.DTCs {
			want[d.TempAddress.Address] = true
		}
		got := map[[3]byte]bool{}
		for _, req := range conn.requests {
			if len(req) > 20 {
				t.Errorf("request has %d addresses, want at most 20", len(req))
			}
			for _, a := range req {
				if got[a] {
					t.Errorf("address %x requested more than once", a)
				}
				got[a] = true
			}
		}
		if len(got) != len(want) {
			t.Fatalf("requested %d distinct addresses, want %d", len(got), len(want))
		}
	})

	t.Run("Set", func(t *testing.T) {
		conn := &memoryConnection{ram: map[[3]byte]byte{
			d3.TempAddress.Address:   1 << d3.TempAddress.Bit,
			d616.TempAddress.Address: 1 << d616.TempAddress.Bit,
		}}
		dtcs, err := ssm2.ReadSetDTCs(context.Background(), conn)
		if err != nil {
			t.Fatal(err)
		}

		got := dtcIDs(dtcs)
		if len(got) != 2 || got[0] != "D3" || got[1] != "D616" {
			t.Fatalf("got %v, want [D3 D616]", got)
		}
	})

	t.Run("Stored", func(t *testing.T) {
		conn := &memoryConnection{ram: map[[3]byte]byte{
			d1.Address.Address: 1 << d1.Address.Bit,
			// set (temp) DTCs shouldn't be reported as stored
			d3.TempAddress.Address: 1 << d3.TempAddress.Bit,
		}}
		dtcs, err := ssm2.ReadStoredDTCs(context.Background(), conn)
		if err != nil {
			t.Fatal(err)
		}

		got := dtcIDs(dtcs)
		if len(got) != 1 || got[0] != "D1" {
			t.Fatalf("got %v, want [D1]", got)
		}
	})

	t.Run("IgnoresUnsupportedAddresses", func(t *testing.T) {
		conn := &memoryConnection{ram: map[[3]byte]byte{
			d1.TempAddress.Address: 0xff,
		}}
		dtcs, err := ssm2.ReadSetDTCs(context.Background(), conn)
		if err != nil {
			t.Fatal(err)
		}
		if len(dtcs) != 0 {
			t.Fatalf("got %v, want none", dtcIDs(dtcs))
		}
	})

	t.Run("ChecksResponseLength", func(t *testing.T) {
		conn := &memoryConnection{ram: map[[3]byte]byte{}, truncate: 1}
		_, err := ssm2.ReadSetDTCs(context.Background(), conn)
		if !errors.Is(err, ssm2.ErrInvalidResponseLength) {
			t.Fatalf("want ErrInvalidResponseLength. got: %v.", err)
		}
	})
}
