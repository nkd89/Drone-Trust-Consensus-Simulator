package routing

import (
	"drone_trust_sim/config"
	"drone_trust_sim/metrics"
	"drone_trust_sim/models"
	"drone_trust_sim/trust"
	"math"
	"sync"
)

type ClusterManager struct {
	sync.RWMutex
	nodes         []*models.DroneNode
	cfg           *config.SimulatorConfig
	trustManager  *trust.Manager
	clusters      map[int][]*models.DroneNode // clusterID -> members
	nodeToCluster map[int]int                 // nodeID -> clusterID
	clusterHeads  map[int]*models.DroneNode   // clusterID -> CH
}

func NewClusterManager(nodes []*models.DroneNode, cfg *config.SimulatorConfig, tm *trust.Manager) *ClusterManager {
	return &ClusterManager{
		nodes:         nodes,
		cfg:           cfg,
		trustManager:  tm,
		clusters:      make(map[int][]*models.DroneNode),
		nodeToCluster: make(map[int]int),
		clusterHeads:  make(map[int]*models.DroneNode),
	}
}

// ReelectClusterHeads - главный метод, который вызывает соответствующий алгоритм выбора
func (cm *ClusterManager) ReelectClusterHeads(currentTime float64, metrics *metrics.Collector) {
	cm.Lock()
	defer cm.Unlock()

	cm.formClusters()

	newCHState := make(map[int]int)

	for clusterID, members := range cm.clusters {
		if len(members) == 0 {
			continue
		}

		var bestCH *models.DroneNode
		var maxFinalScore float64 = -math.MaxFloat64

		for _, candidate := range members {
			if candidate.Energy < cm.cfg.EnergyMin {
				continue
			}

			var score float64

			// <<< ЛОГИКА ВЫБОРА РАСШИРЕНА >>>
			switch cm.cfg.CHSelectionAlgorithm {
			case "Unified PoRS Consensus":
				const w_unified = 0.8 // Вес репутации и производительности
				const w_topo = 0.2    // Вес топологической центральности

				unifiedScore := cm.trustManager.CalculateUnifiedScore(candidate)
				topoFactor := calculateTopologicalFactor(candidate, members, cm.cfg.CommunicationRadius)

				score = w_unified*unifiedScore + w_topo*topoFactor

			case "BaseBTMSD":
				score = cm.trustManager.CalculateMeanIncomingTrust(candidate.ID)
			case "PoRS":
				score = cm.trustManager.CalculatePoRSScore(candidate)
			case "Blockchain":
				score = cm.trustManager.CalculateBlockchainLeaderScore(candidate)
			}

			if score > maxFinalScore {
				maxFinalScore = score
				bestCH = candidate
			}
		}

		if bestCH != nil {
			cm.clusterHeads[clusterID] = bestCH
			newCHState[clusterID] = bestCH.ID
		}
	}

	cm.updateRoles()
	metrics.RecordCHChange(newCHState)
}

func (cm *ClusterManager) formClusters() {
	// Простой алгоритм кластеризации, похожий на BFS
	cm.clusters = make(map[int][]*models.DroneNode)
	cm.nodeToCluster = make(map[int]int)
	visited := make(map[int]bool)
	clusterCounter := 0

	for _, node := range cm.nodes {
		if visited[node.ID] {
			continue
		}
		clusterCounter++

		queue := []*models.DroneNode{node}
		visited[node.ID] = true

		for len(queue) > 0 {
			currentNode := queue[0]
			queue = queue[1:]

			cm.clusters[clusterCounter] = append(cm.clusters[clusterCounter], currentNode)
			cm.nodeToCluster[currentNode.ID] = clusterCounter

			for _, neighbor := range cm.nodes {
				if !visited[neighbor.ID] && currentNode.Location.Distance(neighbor.Location) <= cm.cfg.CommunicationRadius {
					visited[neighbor.ID] = true
					queue = append(queue, neighbor)
				}
			}
		}
	}
}

func (cm *ClusterManager) updateRoles() {
	for _, n := range cm.nodes {
		n.IsClusterHead = false
		n.ClusterID = -1
	}

	for clusterID, ch := range cm.clusterHeads {
		ch.IsClusterHead = true
		ch.ClusterID = clusterID
		for _, member := range cm.clusters[clusterID] {
			member.ClusterID = clusterID
		}
	}
}

func calculateTopologicalFactor(candidate *models.DroneNode, members []*models.DroneNode, clusterRadius float64) float64 {
	if len(members) <= 1 {
		return 1.0 // Если узел один, он в центре
	}

	var totalDistance float64
	for _, member := range members {
		if member.ID == candidate.ID {
			continue
		}
		totalDistance += candidate.Location.Distance(member.Location)
	}
	avgDistance := totalDistance / float64(len(members)-1)

	// Нормализуем. Чем меньше среднее расстояние, тем выше фактор (ближе к 1)
	factor := 1.0 - (avgDistance / clusterRadius)
	return models.Clamp(factor, 0.0, 1.0)
}

// Getters для безопасного доступа из других пакетов
func (cm *ClusterManager) GetNodeClusterHead(nodeID int) *models.DroneNode {
	cm.RLock()
	defer cm.RUnlock()
	if clusterID, ok := cm.nodeToCluster[nodeID]; ok {
		return cm.clusterHeads[clusterID]
	}
	return nil
}

func (cm *ClusterManager) GetClusterHead(clusterID int) *models.DroneNode {
	cm.RLock()
	defer cm.RUnlock()
	return cm.clusterHeads[clusterID]
}

func (cm *ClusterManager) GetClusterMembers(clusterID int) []*models.DroneNode {
	cm.RLock()
	defer cm.RUnlock()
	return cm.clusters[clusterID]
}

func (cm *ClusterManager) GetClusters() map[int][]*models.DroneNode {
	cm.RLock()
	defer cm.RUnlock()
	return cm.clusters
}
