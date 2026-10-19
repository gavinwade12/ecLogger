// Package logging contains the UI-independent pieces of logging ECU
// parameters: which parameters are logged, running logging sessions, and
// writing results to CSV.
package logging

import (
	"sort"
	"sync"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// LoggedParam describes how a parameter is logged.
type LoggedParam struct {
	LogToFile bool
	LiveLog   bool
	Derived   bool
	Unit      units.Unit
}

// Params manages the set of logged parameters, keyed by parameter ID,
// for concurrency-safe access.
type Params struct {
	mu   sync.RWMutex
	data map[string]LoggedParam
}

// NewParams returns a Params initialized with a copy of data.
func NewParams(data map[string]LoggedParam) *Params {
	p := &Params{data: make(map[string]LoggedParam, len(data))}
	for k, v := range data {
		p.data[k] = v
	}
	return p
}

// Get returns the logged parameter with the given ID.
func (p *Params) Get(id string) (LoggedParam, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	lp, ok := p.data[id]
	return lp, ok
}

// Unit returns the unit of the logged parameter with the given ID.
func (p *Params) Unit(id string) (units.Unit, bool) {
	lp, ok := p.Get(id)
	return lp.Unit, ok
}

// Len returns the number of logged parameters.
func (p *Params) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.data)
}

// Snapshot returns a copy of all the logged parameters.
func (p *Params) Snapshot() map[string]LoggedParam {
	p.mu.RLock()
	defer p.mu.RUnlock()

	m := make(map[string]LoggedParam, len(p.data))
	for k, v := range p.data {
		m[k] = v
	}
	return m
}

// SetLogToFile enables or disables file logging for a parameter. The parameter
// is added when needed and removed once it's neither file nor live logged.
func (p *Params) SetLogToFile(id string, derived bool, unit units.Unit, enabled bool) {
	p.setFlag(id, derived, unit, enabled, func(lp *LoggedParam) *bool { return &lp.LogToFile })
}

// SetLiveLog enables or disables live logging for a parameter. The parameter
// is added when needed and removed once it's neither file nor live logged.
func (p *Params) SetLiveLog(id string, derived bool, unit units.Unit, enabled bool) {
	p.setFlag(id, derived, unit, enabled, func(lp *LoggedParam) *bool { return &lp.LiveLog })
}

func (p *Params) setFlag(id string, derived bool, unit units.Unit, enabled bool, flag func(*LoggedParam) *bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	lp, ok := p.data[id]
	if !ok {
		if !enabled {
			return
		}
		lp = LoggedParam{Derived: derived, Unit: unit}
	}
	*flag(&lp) = enabled

	if !lp.LogToFile && !lp.LiveLog {
		delete(p.data, id)
		return
	}
	p.data[id] = lp
}

// SetUnit sets the unit of a logged parameter. It does nothing if the parameter isn't logged.
func (p *Params) SetUnit(id string, unit units.Unit) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if lp, ok := p.data[id]; ok {
		lp.Unit = unit
		p.data[id] = lp
	}
}

// Lists returns the logged parameters and derived parameters, sorted by ID.
// IDs that aren't known to the ssm2 package are skipped.
func (p *Params) Lists() ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	return p.lists(func(LoggedParam) bool { return true })
}

// FileLists is like Lists, but only returns the parameters logged to file.
func (p *Params) FileLists() ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	return p.lists(func(lp LoggedParam) bool { return lp.LogToFile })
}

func (p *Params) lists(include func(LoggedParam) bool) ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	snapshot := p.Snapshot()
	ids := make([]string, 0, len(snapshot))
	for id, lp := range snapshot {
		if include(lp) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	params := []ssm2.Parameter{}
	derived := []ssm2.DerivedParameter{}
	for _, id := range ids {
		if snapshot[id].Derived {
			if d, ok := ssm2.DerivedParameters[id]; ok {
				derived = append(derived, d)
			}
		} else if param, ok := ssm2.Parameters[id]; ok {
			params = append(params, param)
		}
	}
	return params, derived
}
