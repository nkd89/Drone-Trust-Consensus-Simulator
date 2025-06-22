// Файл: consensus/pors_consensus.go
package consensus

import (
	"drone_trust_sim/models"
	"drone_trust_sim/trust" // Импортируем, чтобы получить доступ к формулам
	"math"
)

// PoRSConsensus реализует интерфейс ConsensusEngine
type PoRSConsensus struct{}

func calculateUnifiedScore(tm *trust.Manager, candidate *models.DroneNode) float64 {
	const w_pors = 0.6 // Вес текущей производительности
	const w_rf = 0.4   // Вес долгосрочной репутации (стабильности)

	porsScore := tm.CalculatePoRSScore(candidate)

	var rfScore float64
	if candidate.ConsensusRounds > 0 {
		rfScore = float64(candidate.ValidBlocksProposed) / float64(candidate.ConsensusRounds)
	} else {
		rfScore = 0.5 // Нейтральное значение для новичков
	}

	return w_pors*porsScore + w_rf*rfScore
}

// Run эмулирует один раунд консенсуса на основе PoRS.
func (p *PoRSConsensus) Run(currentTime float64, members []*models.DroneNode, simState SimulatorState) (float64, *models.Block) {
	if len(members) == 0 {
		return 0, nil
	}

	cfg := simState.GetConfig()
	// Получаем доступ к TrustManager через интерфейс, который должен его предоставлять.
	// Нам нужно будет расширить интерфейс SimulatorState.
	tm, ok := simState.GetTrustManager().(*trust.Manager)
	if !ok {
		// Не удалось получить доступ к менеджеру доверия, консенсус невозможен
		return 0, nil
	}

	var bestProposer *models.DroneNode
	var maxScore float64 = -math.MaxFloat64

	for _, candidate := range members {
		if candidate.Energy < cfg.EnergyMin {
			continue
		}

		score := calculateUnifiedScore(tm, candidate)
		if score > maxScore {
			maxScore = score
			bestProposer = candidate
		}
	}

	if bestProposer == nil {
		return 0, nil // Не удалось выбрать лидера
	}

	// Задержка консенсуса в этой модели - это в основном время на коммуникацию
	// и голосование. Сделаем ее небольшой и фиксированной.
	latency := 0.2 // 200ms

	blockID := simState.GetNextPacketID()
	block := &models.Block{
		ID:         blockID,
		ProposerID: bestProposer.ID,
		Timestamp:  currentTime + latency,
	}

	// Можно добавить "бонус" к репутации лидера, но это усложнит модель доверия.
	// Пока оставим без бонуса для чистоты эксперимента.

	return latency, block
}
