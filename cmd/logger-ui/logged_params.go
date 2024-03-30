package main

import (
	"sync"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

type LoggedParam struct {
	LogToFile bool
	LiveLog   bool
	Derived   bool
	Unit      units.Unit
}

// LoggedParams manages a set of logged params for conccurency-safe access.
type LoggedParams struct {
	data map[string]*LoggedParam
	mu   sync.RWMutex
}

func NewLoggedParams(data map[string]*LoggedParam) *LoggedParams {
	return &LoggedParams{data: data}
}

func (p *LoggedParams) Remove(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.data, key)
}

func (p *LoggedParams) Update(key string, update func(*LoggedParam)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	update(p.data[key])
}

func (p *LoggedParams) UpdateOrAdd(key string, update func(*LoggedParam), add *LoggedParam) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.data[key] != nil {
		update(p.data[key])
	} else {
		p.data[key] = add
	}
}

func (p *LoggedParams) Get(key string) *LoggedParam {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data[key]
}

func (p *LoggedParams) CopyData() map[string]*LoggedParam {
	p.mu.RLock()
	defer p.mu.RUnlock()

	m := make(map[string]*LoggedParam)
	for k, v := range p.data {
		m[k] = v
	}
	return m
}

func (p *LoggedParams) CurrentLists() ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	params := []ssm2.Parameter{}
	derivedParams := []ssm2.DerivedParameter{}
	LoggedParams := p.CopyData()
	for id, p := range LoggedParams {
		if p.Derived {
			derivedParams = append(derivedParams, ssm2.DerivedParameters[id])
		} else {
			params = append(params, ssm2.Parameters[id])
		}
	}

	return params, derivedParams
}
