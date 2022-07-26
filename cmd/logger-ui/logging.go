package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
)

var liveLogContainer *fyne.Container

func loggingContainer() fyne.CanvasObject {
	liveLogContainer = container.New(layout.NewGridLayout(3))
	return container.NewVScroll(liveLogContainer)
}

type liveLogModel struct {
	Name                string
	CurrentValueBinding binding.String

	MaxValue        float32
	MaxValueBinding binding.String
	MinValue        float32
	MinValueBinding binding.String
}

func newLiveLogModel(name string) *liveLogModel {
	m := &liveLogModel{
		Name:                name,
		CurrentValueBinding: binding.NewString(),
		MaxValueBinding:     binding.NewString(),
		MinValueBinding:     binding.NewString(),
	}
	m.CurrentValueBinding.Set("0")
	m.MaxValueBinding.Set("0")
	m.MinValueBinding.Set("0")
	return m
}

func (m *liveLogModel) Update(val float32) {
	f := strconv.FormatFloat(float64(val), 'f', 2, 32)
	m.CurrentValueBinding.Set(f)

	if val > m.MaxValue {
		m.MaxValue = val
		m.MaxValueBinding.Set(f)
	}
	if val < m.MinValue {
		m.MinValue = val
		m.MinValueBinding.Set(f)
	}
}

var (
	liveLogModels   []*liveLogModel
	liveLogModelsMu sync.Mutex
)

func updateLiveLogParameters() {
	liveLogContainer.RemoveAll()

	liveLogModelsMu.Lock()
	liveLogModels = []*liveLogModel{}
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

		liveLogModels = append(liveLogModels, newLiveLogModel(name))
	}
	sort.Sort(sortableLiveLogModels(liveLogModels))
	liveLogModelLen := len(liveLogModels)
	liveLogModelsMu.Unlock()

	for _, m := range liveLogModels {
		liveLogContainer.Objects = append(liveLogContainer.Objects,
			container.NewVBox(
				widget.NewLabel(m.Name),
				widget.NewLabelWithData(m.CurrentValueBinding),
				container.NewHBox(
					widget.NewLabelWithData(m.MinValueBinding),
					widget.NewLabel("/"),
					widget.NewLabelWithData(m.MaxValueBinding),
				),
			))
	}

	liveLogContainer.Refresh()

	if liveLogModelLen > 0 {
		if stopLogging != nil {
			fmt.Println("stop logging")
			stopLogging()
			stopLogging = nil
		}

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

	fmt.Println("sending read address request")
	for {
		_, err := conn.SendReadAddressesRequest(ctx, addressesToRead, true)
		if err == nil {
			break
		}
		fmt.Println(err.Error())
	}

	fmt.Println("starting packet read")
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
			addrIndex, liveLogIndex := 0, 0
			values := make(map[string]ssm2.ParameterValue)
			liveLogModelsMu.Lock()
			loggedParams := readOnlyLoggedParams()
			for _, param := range params {
				val := param.Value(data[addrIndex : addrIndex+param.Address.Length])
				values[param.Id] = val
				addrIndex += param.Address.Length

				if loggedParams[param.Id].LiveLog {
					liveLogModels[liveLogIndex].Update(val.Value)
					liveLogIndex++
				}
			}
			for _, param := range derivedParams {
				val, err := param.Value(values)
				if err == nil {
					values[param.Id] = *val
				}

				if loggedParams[param.Id].LiveLog {
					liveLogIndex++
					if err == nil {
						liveLogModels[liveLogIndex].Update(val.Value)
					}
				}
			}
			liveLogModelsMu.Unlock()
		}
	}
}

type sortableLiveLogModels []*liveLogModel

func (a sortableLiveLogModels) Len() int           { return len(a) }
func (a sortableLiveLogModels) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortableLiveLogModels) Less(i, j int) bool { return a[i].Name < a[j].Name }
