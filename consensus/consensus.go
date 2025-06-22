package consensus

import (
	"drone_trust_sim/config"
	"drone_trust_sim/models"
	"log"
	"sync"
)

type TrustManagerProvider interface {
	CalculatePoRSScore(candidate *models.DroneNode) float64
}

// SimulatorState определяет методы, которые консенсусу нужны от симулятора,
// чтобы избежать циклического импорта.
type SimulatorState interface {
	GetClusterHead(clusterID int) *models.DroneNode
	GetClusterMembers(clusterID int) []*models.DroneNode
	GetConfig() *config.SimulatorConfig
	GetNextPacketID() int
	GetTrustManager() TrustManagerProvider
}

// ConsensusEngine - интерфейс для любого механизма консенсуса
type ConsensusEngine interface {
	Run(currentTime float64, members []*models.DroneNode, simState SimulatorState) (float64, *models.Block)
}

// RunConsensusRound - функция, запускаемая в горутине для эмуляции раунда консенсуса
// <<< ИЗМЕНЕНО: принимает интерфейс, а не конкретный симулятор >>>
func RunConsensusRound(currentTime float64, clusterID int, sim SimulatorState, wg *sync.WaitGroup) {
	defer wg.Done()

	ch := sim.GetClusterHead(clusterID)
	members := sim.GetClusterMembers(clusterID)
	cfg := sim.GetConfig()

	if ch == nil || len(members) <= 1 {
		return // Консенсус невозможен
	}

	// log.Printf("t=%.2f: [Кластер %d] Запуск консенсуса (%s) среди %d узлов. Лидер: Дрон %d",
	// 	currentTime, clusterID, cfg.ConsensusType, len(members), ch.ID)

	var engine ConsensusEngine
	switch cfg.ConsensusType {
	case "PBFT":
		engine = &PBFT{}
	case "PoW":
		engine = &PoW{}
	case "PoRS_Consensus":
		engine = &PoRSConsensus{}
	default:
		log.Printf("Неизвестный тип консенсуса: %s", cfg.ConsensusType)
		return
	}

	_, block := engine.Run(currentTime, members, sim)

	if block == nil {
		log.Printf("t=%.2f: [Кластер %d] Консенсус не удался.", currentTime, clusterID)
		return
	}

	// Обновление состояния после консенсуса
	for _, node := range members {
		node.Mutex.Lock()
		node.Energy -= cfg.EnergyConsensus
		node.ConsensusRounds++
		if node.ID == block.ProposerID {
			node.ValidBlocksProposed++
		}
		node.Mutex.Unlock()
	}

	// log.Printf("t=%.2f: [Кластер %d] Консенсус завершен. Задержка: %.3f с. Новый блок #%d создан Дроном %d",
	// 	currentTime+latency, clusterID, latency, block.ID, block.ProposerID)
}
