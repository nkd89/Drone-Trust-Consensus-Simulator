// Файл: consensus/pow.go
package consensus

import (
	"drone_trust_sim/models"
	"math/rand"
)

// PoW реализует интерфейс ConsensusEngine
type PoW struct{}

// Run эмулирует один раунд PoW, где вероятность выигрыша зависит от мощности.
func (p *PoW) Run(currentTime float64, members []*models.DroneNode, simState SimulatorState) (float64, *models.Block) {
	if len(members) == 0 {
		return 0, nil
	}

	cfg := simState.GetConfig()

	// --- Выбор "победителя" пропорционально вычислительной мощности ---
	var totalPower float64
	for _, node := range members {
		totalPower += node.ComputationalPower
	}

	if totalPower == 0 { // Защита от деления на ноль
		return 0, nil
	}

	// "Рулетка": выбираем случайное число от 0 до totalPower
	pick := rand.Float64() * totalPower

	var winner *models.DroneNode
	var currentPowerSum float64
	for _, node := range members {
		currentPowerSum += node.ComputationalPower
		if pick < currentPowerSum {
			winner = node
			break
		}
	}
	// Если из-за ошибок округления никто не выбран, берем последнего
	if winner == nil {
		winner = members[len(members)-1]
	}
	// --- Конец выбора победителя ---

	// Задержка - это время майнинга.
	latency := cfg.PoWMiningTime + (rand.Float64()-0.5)*cfg.PoWMiningTime*0.2

	blockID := simState.GetNextPacketID()
	block := &models.Block{
		ID:         blockID,
		ProposerID: winner.ID,
		Timestamp:  currentTime + latency,
	}

	return latency, block
}
