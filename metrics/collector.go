package metrics

import (
	"sync"
)

// Collector собирает сырые метрики во время симуляции
type Collector struct {
	sync.Mutex
	PacketsSent         int
	PacketsDelivered    int
	TotalDelay          float64
	TotalEnergyConsumed float64
	TotalHops           int
	CHChanges           int
	LastCHState         map[int]int // clusterID -> chID
}

func NewCollector() *Collector {
	return &Collector{
		LastCHState: make(map[int]int),
	}
}

func (mc *Collector) RecordPacketSent() {
	mc.Lock()
	defer mc.Unlock()
	mc.PacketsSent++
}

func (mc *Collector) RecordPacketDelivered(delay float64) {
	mc.Lock()
	defer mc.Unlock()
	mc.PacketsDelivered++
	mc.TotalDelay += delay
}

func (mc *Collector) RecordEnergyConsumed(energy float64) {
	mc.Lock()
	defer mc.Unlock()
	mc.TotalEnergyConsumed += energy
}

func (mc *Collector) RecordCHChange(currentCHState map[int]int) {
	mc.Lock()
	defer mc.Unlock()

	if len(mc.LastCHState) == 0 {
		mc.LastCHState = currentCHState
		return
	}

	changes := 0
	for clusterID, newCHID := range currentCHState {
		if oldCHID, ok := mc.LastCHState[clusterID]; ok {
			if oldCHID != newCHID {
				changes++
			}
		} else {
			changes++ // Новый кластер
		}
	}
	mc.CHChanges += changes
	mc.LastCHState = currentCHState
}
