package trust

import (
	"drone_trust_sim/config"
	"drone_trust_sim/models"
	"sync"
)

type Manager struct {
	sync.RWMutex
	nodes          []*models.DroneNode
	cfg            *config.SimulatorConfig
	trustMatrix    [][]float64
	lastUpdateTime [][]float64
}

func NewManager(nodes []*models.DroneNode, cfg *config.SimulatorConfig) *Manager {
	n := len(nodes)
	tm := &Manager{
		nodes:          nodes,
		cfg:            cfg,
		trustMatrix:    make([][]float64, n),
		lastUpdateTime: make([][]float64, n),
	}
	for i := range tm.trustMatrix {
		tm.trustMatrix[i] = make([]float64, n)
		tm.lastUpdateTime[i] = make([]float64, n)
		for j := range tm.trustMatrix[i] {
			if i == j {
				tm.trustMatrix[i][j] = 1.0 // Доверие к себе
			} else {
				tm.trustMatrix[i][j] = cfg.InitialTrustValue
			}
		}
	}
	return tm
}

// RecordInteraction - главный метод для обновления доверия после взаимодействия
func (tm *Manager) RecordInteraction(observerID, targetID int, result models.InteractionResult, currentTime float64) {
	tm.Lock()
	defer tm.Unlock()

	// Новое, более гибкое условие
	switch tm.cfg.TrustModel {
	case "TrustByDefault":
		tm.updateTrustByDefault_unsafe(observerID, targetID, result, currentTime)
	case "Complex":
		tm.updateComprehensiveTrust_unsafe(observerID, targetID, result, currentTime)
	default: // "Simple"
		success := result == models.InteractionSuccess
		tm.updateSimpleTrust_unsafe(observerID, targetID, success)
	}
}

// updateSimpleTrust_unsafe - внутренняя версия, работает без блокировки
func (tm *Manager) updateSimpleTrust_unsafe(observerID, targetID int, success bool) {
	oldTrust := tm.trustMatrix[observerID][targetID]
	observation := 0.0
	if success {
		observation = 1.0
	}
	newTrust := (1-tm.cfg.AlphaTrust)*oldTrust + tm.cfg.AlphaTrust*observation
	tm.trustMatrix[observerID][targetID] = models.Clamp(newTrust, 0, 1)
}

// updateComprehensiveTrust_unsafe - внутренняя версия, работает без блокировки
func (tm *Manager) updateComprehensiveTrust_unsafe(observerID, targetID int, result models.InteractionResult, currentTime float64) {
	// 1. Рассчитываем прямой trust (T_d)
	// <<< ИЗМЕНЕНО: передаем 'tm' в дочерние функции, чтобы они могли использовать его состояние >>>
	directTrust := calculateDirectTrust_unsafe(tm, observerID, targetID, result)

	// 2. Рассчитываем рекомендованный trust (T_re)
	recommendedTrust := calculateRecommendedTrust_unsafe(tm, observerID, targetID)

	// 3. Рассчитываем исторический trust (T_h)
	historicalTrust := calculateHistoricalTrust_unsafe(tm, observerID, targetID, currentTime)

	// 4. Агрегируем все в комплексное доверие (T_total)
	totalTrust := calculateTotalTrust(directTrust, recommendedTrust, historicalTrust)

	tm.trustMatrix[observerID][targetID] = models.Clamp(totalTrust, 0, 1)
	tm.lastUpdateTime[observerID][targetID] = currentTime
}

// GetTrust - публичный, потокобезопасный метод для чтения
func (tm *Manager) GetTrust(observerID, targetID int) float64 {
	tm.RLock() // <<< Блокировка на ЧТЕНИЕ
	defer tm.RUnlock()
	return tm.trustMatrix[observerID][targetID]
}
