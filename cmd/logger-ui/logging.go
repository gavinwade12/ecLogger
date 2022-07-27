package main

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
)

var liveLogContainer *fyne.Container

func loggingContainer() fyne.CanvasObject {
	startBtn := widget.NewToolbarAction(theme.MediaPlayIcon(), nil)
	stopBtn := widget.NewToolbarAction(theme.MediaStopIcon(), nil)
	toolbar := widget.NewToolbar()

	startBtn.OnActivated = func() {
		// TODO: start file logging
		toolbar.Items = []widget.ToolbarItem{}
		toolbar.Append(stopBtn)
	}
	stopBtn.OnActivated = func() {
		// TODO: stop file logging
		toolbar.Items = []widget.ToolbarItem{}
		toolbar.Append(startBtn)
	}
	toolbar.Append(startBtn)

	liveLogContainer = container.New(layout.NewGridLayout(3))
	return container.NewBorder(toolbar, nil, nil, nil, container.NewVScroll(liveLogContainer))
}

type liveLogModel struct {
	Id                  string
	Name                string
	CurrentValueBinding binding.String
	UnitBinding         binding.String

	MaxValue        float32
	MaxValueBinding binding.String
	MinValue        float32
	MinValueBinding binding.String
}

func newLiveLogModel(id, name string) *liveLogModel {
	m := &liveLogModel{
		Id:                  id,
		Name:                name,
		CurrentValueBinding: binding.NewString(),
		UnitBinding:         binding.NewString(),
		MaxValueBinding:     binding.NewString(),
		MinValueBinding:     binding.NewString(),
	}
	m.CurrentValueBinding.Set("0")
	m.MaxValueBinding.Set("0")
	m.MinValueBinding.Set("0")
	return m
}

func (m *liveLogModel) Update(val ssm2.ParameterValue) {
	f := strconv.FormatFloat(float64(val.Value), 'f', 2, 32)
	m.CurrentValueBinding.Set(f)
	m.UnitBinding.Set(string(val.Unit))

	if val.Value > m.MaxValue {
		m.MaxValue = val.Value
		m.MaxValueBinding.Set(f)
	}
	if val.Value < m.MinValue {
		m.MinValue = val.Value
		m.MinValueBinding.Set(f)
	}
}

var (
	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex
)

func updateLiveLogParameters() {
	liveLogContainer.RemoveAll()

	if stopLogging != nil {
		stopLogging()
		stopLogging = nil
	}

	liveLogModelsMu.Lock()
	liveLogModels = []*liveLogModel{}
	if conn == nil {
		liveLogModelsMu.Unlock()
		liveLogContainer.Refresh()
		return
	}

	loggedParams := readOnlyLoggedParams()
	for id, param := range loggedParams {
		if !param.LiveLog {
			continue
		}

		var name string
		if param.Derived {
			name = ssm2.DerivedParameters[id].Name
		} else {
			name = ssm2.Parameters[id].Name
		}

		liveLogModels = append(liveLogModels, newLiveLogModel(id, name))
	}
	sort.Sort(sortableLiveLogModels(liveLogModels))
	liveLogModelsLen := len(liveLogModels)
	liveLogModelsMu.Unlock()

	for _, m := range liveLogModels {
		liveLogContainer.Objects = append(liveLogContainer.Objects,
			container.NewVBox(
				widget.NewLabel(m.Name),
				container.NewHBox(
					widget.NewLabelWithData(m.CurrentValueBinding),
					widget.NewLabelWithData(m.UnitBinding),
				),
				container.NewHBox(
					widget.NewLabelWithData(m.MinValueBinding),
					widget.NewLabel("/"),
					widget.NewLabelWithData(m.MaxValueBinding),
				),
			))
	}

	liveLogContainer.Refresh()

	if liveLogModelsLen > 0 {
		var ctx context.Context
		ctx, stopLogging = context.WithCancel(context.Background())
		go startLogging(ctx)
	}
}

var stopLogging context.CancelFunc

func startLogging(ctx context.Context) {
	params := []ssm2.Parameter{}
	derivedParams := []ssm2.DerivedParameter{}
	addressesToRead := [][3]byte{}
	loggedParams := readOnlyLoggedParams()
	for id, p := range loggedParams {
		if p.Derived {
			derivedParams = append(derivedParams, ssm2.DerivedParameters[id])
			continue
		}

		param := ssm2.Parameters[id]
		for i := 0; i < param.Address.Length; i++ {
			addressesToRead = append(addressesToRead, param.Address.Add(i))
		}
		params = append(params, param)
	}

	for {
		_, err := conn.SendReadAddressesRequest(ctx, addressesToRead, true)
		if err == nil {
			break
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			packet, err := conn.NextPacket(ctx)
			if err != nil {
				logger.Debug(err.Error())
				continue
			}

			data := packet.Data()
			addrIndex := 0
			values := make(map[string]ssm2.ParameterValue)
			loggedParams := readOnlyLoggedParams()
			for _, param := range params {
				lp := loggedParams[param.Id]
				if lp == nil {
					continue
				}

				val := param.Value(data[addrIndex : addrIndex+param.Address.Length])
				u := lp.Unit
				if u != val.Unit {
					vval, err := val.ConvertTo(u)
					if err != nil {
						logger.Debugf("converting %s from %s to %s: %v", param.Id, val.Unit, u, err)
						continue
					}
					val = *vval
				}
				values[param.Id] = val
				addrIndex += param.Address.Length
			}
			for _, param := range derivedParams {
				lp := loggedParams[param.Id]
				if lp == nil {
					continue
				}

				val, err := param.Value(values)
				if err == nil {
					u := lp.Unit
					if u != val.Unit {
						val, err = val.ConvertTo(u)
						if err != nil {
							logger.Debugf("converting %s from %s to %s: %v", param.Id, val.Unit, u, err)
							continue
						}
					}
					values[param.Id] = *val
				}
			}
			liveLogModelsMu.Lock()
			for _, m := range liveLogModels {
				m.Update(values[m.Id])
			}
			liveLogModelsMu.Unlock()
		}
	}
}

type sortableLiveLogModels []*liveLogModel

func (a sortableLiveLogModels) Len() int           { return len(a) }
func (a sortableLiveLogModels) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableLiveLogModels) Less(i, j int) bool { return a[i].Name < a[j].Name }
