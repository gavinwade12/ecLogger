package logging_test

import (
	"sync"
	"testing"

	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/units"
)

func TestParamsFlags(t *testing.T) {
	p := logging.NewParams(nil)

	p.SetLogToFile("P8", false, units.RPM, true)
	lp, ok := p.Get("P8")
	if !ok || !lp.LogToFile || lp.LiveLog || lp.Unit != units.RPM {
		t.Fatalf("after enabling file logging: %+v, %v", lp, ok)
	}

	p.SetLiveLog("P8", false, units.RPM, true)
	if lp, _ := p.Get("P8"); !lp.LogToFile || !lp.LiveLog {
		t.Fatalf("after enabling live logging: %+v", lp)
	}

	// still live logged, so it's kept
	p.SetLogToFile("P8", false, units.RPM, false)
	if lp, ok := p.Get("P8"); !ok || lp.LogToFile || !lp.LiveLog {
		t.Fatalf("after disabling file logging: %+v, %v", lp, ok)
	}

	// no longer logged at all, so it's removed
	p.SetLiveLog("P8", false, units.RPM, false)
	if _, ok := p.Get("P8"); ok {
		t.Fatal("expected P8 to be removed")
	}

	// disabling an unknown parameter doesn't add it
	p.SetLiveLog("P9", false, units.MPH, false)
	if p.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", p.Len())
	}
}

func TestParamsSetUnit(t *testing.T) {
	p := logging.NewParams(map[string]logging.LoggedParam{
		"P9": {LiveLog: true, Unit: units.MPH},
	})

	p.SetUnit("P9", units.KMH)
	if lp, _ := p.Get("P9"); lp.Unit != units.KMH {
		t.Fatalf("Unit = %s, want km/h", lp.Unit)
	}

	p.SetUnit("P8", units.RPM) // not logged
	if _, ok := p.Get("P8"); ok {
		t.Fatal("SetUnit shouldn't add parameters")
	}
}

func TestParamsUnit(t *testing.T) {
	p := logging.NewParams(map[string]logging.LoggedParam{
		"P9": {LiveLog: true, Unit: units.MPH},
	})

	if u, ok := p.Unit("P9"); !ok || u != units.MPH {
		t.Fatalf("Unit(P9) = %s, %v; want mph, true", u, ok)
	}
	if _, ok := p.Unit("P8"); ok {
		t.Fatal("Unit(P8) reported an unlogged parameter")
	}
}

func TestParamsCopiesInput(t *testing.T) {
	data := map[string]logging.LoggedParam{"P9": {LiveLog: true}}
	p := logging.NewParams(data)
	delete(data, "P9")
	if _, ok := p.Get("P9"); !ok {
		t.Fatal("NewParams should copy its input")
	}

	snapshot := p.Snapshot()
	delete(snapshot, "P9")
	if _, ok := p.Get("P9"); !ok {
		t.Fatal("Snapshot should return a copy")
	}
}

func TestParamsLists(t *testing.T) {
	p := logging.NewParams(map[string]logging.LoggedParam{
		"P9":    {LiveLog: true},
		"P12":   {LiveLog: true},
		"P8":    {LogToFile: true},
		"P200":  {LiveLog: true, Derived: true},
		"P9999": {LiveLog: true}, // unknown, e.g. from an old config
	})

	params, derived := p.Lists()
	var ids []string
	for _, param := range params {
		ids = append(ids, param.Id)
	}
	if len(ids) != 3 || ids[0] != "P12" || ids[1] != "P8" || ids[2] != "P9" {
		t.Fatalf("params = %v, want [P12 P8 P9]", ids)
	}
	if len(derived) != 1 || derived[0].Id != "P200" {
		t.Fatalf("derived = %v, want [P200]", derived)
	}
}

func TestParamsConcurrentAccess(t *testing.T) {
	p := logging.NewParams(map[string]logging.LoggedParam{"P9": {LiveLog: true, Unit: units.MPH}})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			p.SetUnit("P9", units.KMH)
		}()
		go func() {
			defer wg.Done()
			_ = p.Snapshot()["P9"].Unit
		}()
	}
	wg.Wait()
}
