package trust

import (
	"drone_trust_sim/models"
	"math"
)

// --- Функции для выбора CH ---

// CalculateMeanIncomingTrust (для BaseBTMSD)
func (tm *Manager) CalculateMeanIncomingTrust(candidateID int) float64 {
	// Эта функция вызывается извне, поэтому она должна быть безопасной
	// Но она только читает, так что можно оставить как есть, если
	// GetTrust использует RLock. Однако, чтобы избежать потенциальных
	// проблем, лучше сделать явную блокировку на чтение на все время расчета.
	tm.RLock()
	defer tm.RUnlock()

	sum := 0.0
	for i := range tm.nodes {
		if i == candidateID {
			continue
		}
		// Используем прямой доступ к матрице, т.к. уже под RLock
		sum += tm.trustMatrix[i][candidateID]
	}
	if len(tm.nodes) <= 1 {
		return 0.0
	}
	return sum / float64(len(tm.nodes)-1)
}

// CalculatePoRSScore (для PoRS)
func (tm *Manager) CalculatePoRSScore(candidate *models.DroneNode) float64 {
	// Веса для PoRS
	const wTrust, wPDR, wEnergy = 0.5, 0.3, 0.2

	trustScore := tm.CalculateMeanIncomingTrust(candidate.ID)

	var pdrScore float64
	if candidate.PacketsSent > 0 {
		pdrScore = float64(candidate.PacketsDelivered) / float64(candidate.PacketsSent)
	} else {
		pdrScore = 1.0 // Нейтрально, если еще не отправлял
	}

	energyScore := candidate.Energy / tm.cfg.InitialEnergy

	return wTrust*trustScore + wPDR*pdrScore + wEnergy*energyScore
}

// CalculateBlockchainLeaderScore (для алгоритма из статьи, формула 9)
func (tm *Manager) CalculateBlockchainLeaderScore(candidate *models.DroneNode) float64 {
	// Веса E1, E2. По статье, [0.5, 0.5] - хороший выбор
	const E1, E2 = 0.5, 0.5

	// RF - Поведенческий фактор
	rf := calculateRF(candidate)

	// FF - Вычислительный фактор
	ff := calculateFF(candidate, tm.nodes)

	return E1*rf + E2*ff
}

func calculateRF(node *models.DroneNode) float64 {
	// Упрощенная версия формулы (7)
	// Базовая репутация + бонус за хорошие действия
	// Мы не храним историю, поэтому используем простую пропорцию
	if node.ConsensusRounds == 0 {
		return 0.5 // Начальная репутация
	}
	// Доля успешных участий
	reputation := float64(node.ValidBlocksProposed) / float64(node.ConsensusRounds)
	return models.Clamp(reputation, 0, 1)
}

func calculateFF(node *models.DroneNode, allNodes []*models.DroneNode) float64 {
	// Формула (8)
	maxPower := 0.0
	for _, n := range allNodes {
		if n.ComputationalPower > maxPower {
			maxPower = n.ComputationalPower
		}
	}
	if maxPower == 0 {
		return 0
	}
	return node.ComputationalPower / maxPower
}

func calculateDirectTrust_unsafe(tm *Manager, obsID, tgtID int, result models.InteractionResult) float64 {
	oldTrust := tm.trustMatrix[obsID][tgtID]
	alpha := tm.cfg.AlphaTrust

	var observation float64

	switch result {
	case models.InteractionSuccess:
		observation = 1.0
	case models.Failure_MaliciousDrop:
		observation = 0.0
		alpha = 0.7
	default:
		return oldTrust
	}

	newTrust := (1-alpha)*oldTrust + alpha*observation
	return models.Clamp(newTrust, 0.0, 1.0)
}

func calculateRecommendedTrust_unsafe(tm *Manager, obsID, tgtID int) float64 {
	// Формула (1)
	// <<< НЕ ИСПОЛЬЗУЕТ БЛОКИРОВКИ, т.к. вызывается из-под Lock >>>
	numerator := 0.0
	denominator := 0.0

	for k := range tm.nodes {
		if k == obsID || k == tgtID {
			continue
		}

		trustInRecommender := tm.trustMatrix[obsID][k]
		recommendation := tm.trustMatrix[k][tgtID]

		numerator += trustInRecommender * recommendation
		denominator += trustInRecommender
	}

	if denominator == 0 {
		return tm.trustMatrix[obsID][tgtID]
	}
	return numerator / denominator
}

func calculateHistoricalTrust_unsafe(tm *Manager, obsID, tgtID int, currentTime float64) float64 {
	// Формула (2) - динамический фактор затухания
	// <<< НЕ ИСПОЛЬЗУЕТ БЛОКИРОВКИ >>>
	lastUpdate := tm.lastUpdateTime[obsID][tgtID]
	timeElapsed := currentTime - lastUpdate

	decayFactor := math.Exp(-tm.cfg.LambdaDecay * timeElapsed)
	historicalTrust := tm.trustMatrix[obsID][tgtID]

	return historicalTrust * decayFactor
}

// calculateTotalTrust (без изменений, т.к. это чистая функция)
func calculateTotalTrust(direct, recommended, historical float64) float64 {
	const wDirect, wRecommended, wHistorical = 0.5, 0.3, 0.2
	return wDirect*direct + wRecommended*recommended + wHistorical*historical
}

func (tm *Manager) CalculateUnifiedScore(candidate *models.DroneNode) float64 {
	// <<< ИЗМЕНЕНЫ ВЕСА: отдаем приоритет стабильности >>>
	const w_pors = 0.3 // Вес текущей производительности
	const w_rf = 0.7   // Вес долгосрочной репутации (стабильности)

	porsScore := tm.CalculatePoRSScore(candidate)

	var rfScore float64
	// RF - это репутация в консенсусе
	if candidate.ConsensusRounds > 0 {
		rfScore = float64(candidate.ValidBlocksProposed) / float64(candidate.ConsensusRounds)
	} else {
		rfScore = 0.5 // Нейтральное значение для новичков
	}

	return w_pors*porsScore + w_rf*rfScore
}

func calculateTrustByDefault_unsafe(tm *Manager, obsID, tgtID int, result models.InteractionResult, currentTime float64) float64 {
	oldTrust := tm.trustMatrix[obsID][tgtID]

	switch result {
	case models.Failure_MaliciousDrop:
		// Резко наказываем за доказанный злой умысел
		// Можно использовать экспоненциальное наказание
		return oldTrust * 0.5 // Каждый сброс режет доверие вдвое

	case models.InteractionSuccess:
		// Если узел был наказан, даем ему шанс медленно восстановиться
		if oldTrust < tm.cfg.InitialTrustValue {
			// Медленное восстановление, например, +0.01 за каждый успешный пакет
			return models.Clamp(oldTrust+0.01, 0.0, tm.cfg.InitialTrustValue)
		}
		// Если доверие уже на максимуме, ничего не делаем
		return oldTrust

	default:
		// Для всех остальных случаев (нейтральные сбои, отсутствие взаимодействий)
		// НЕ МЕНЯЕМ ДОВЕРИЕ ВОВСЕ.
		return oldTrust
	}
}

// updateTrustByDefault_unsafe - обертка для новой модели
func (tm *Manager) updateTrustByDefault_unsafe(observerID, targetID int, result models.InteractionResult, currentTime float64) {
	// Для этой модели нам не нужны рекомендации, так как они могут быть источником FP.
	// Полагаемся только на прямое наблюдение.
	newTrust := calculateTrustByDefault_unsafe(tm, observerID, targetID, result, currentTime)
	tm.trustMatrix[observerID][targetID] = newTrust
}
