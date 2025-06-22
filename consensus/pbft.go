// Файл: consensus/pbft.go
package consensus

import (
	"drone_trust_sim/models"
)

// PBFT реализует интерфейс ConsensusEngine
type PBFT struct{}

// Run эмулирует один раунд PBFT.
// Он возвращает вычисленную задержку и созданный блок.
// Использует SimulatorState для доступа к конфигурации и другим данным симулятора.
func (p *PBFT) Run(currentTime float64, members []*models.DroneNode, simState SimulatorState) (float64, *models.Block) {
	numMembers := len(members)
	// Для PBFT требуется как минимум 3f+1 узлов для устойчивости к f злоумышленникам.
	// Упрощенно говоря, это означает, что нам нужно более 2/3 сети для достижения консенсуса.
	// Здесь мы требуем как минимум 4 узла для работы.
	if numMembers < 4 {
		// Недостаточно узлов для консенсуса
		return 0, nil // Возвращаем nil блок, если консенсус не удался
	}

	cfg := simState.GetConfig()

	// Задержка в PBFT сильно зависит от коммуникации.
	// Эмулируем это как базовую задержку + задержку, зависящую от числа участников.
	// Это очень упрощенная модель.
	latency := cfg.PBFTBaseLatency + (float64(numMembers) * 0.05) // 50 мс на каждого участника

	// В реальном PBFT лидер (Proposer) выбирается по кругу (round-robin) или по репутации.
	// Здесь, для простоты, мы предполагаем, что лидер консенсуса - это первый узел в списке,
	// который обычно является Главой Кластера (CH).
	proposer := members[0]

	// Получаем уникальный ID для нового блока от симулятора
	blockID := simState.GetNextPacketID()

	block := &models.Block{
		ID:         blockID,
		ProposerID: proposer.ID,
		Timestamp:  currentTime + latency,
	}

	return latency, block
}
