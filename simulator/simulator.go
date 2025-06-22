package simulator

import (
	"container/heap"
	"drone_trust_sim/config"
	"drone_trust_sim/consensus"
	"drone_trust_sim/metrics"
	"drone_trust_sim/models"
	"drone_trust_sim/routing"
	"drone_trust_sim/trust"
	"log"
	"math/rand"
	"sync"
	"time"
)

type Simulator struct {
	Cfg            *config.SimulatorConfig
	Nodes          []*models.DroneNode
	CurrentTime    float64
	EventQueue     PriorityQueue
	EventQueueMux  sync.Mutex
	Metrics        *metrics.Collector
	TrustManager   *trust.Manager
	ClusterManager *routing.ClusterManager
	Wg             sync.WaitGroup // Для ожидания завершения всех горутин
	PacketCounter  int
}

func NewSimulator(cfg *config.SimulatorConfig) *Simulator {
	rand.Seed(time.Now().UnixNano())
	s := &Simulator{
		Cfg:        cfg,
		Metrics:    metrics.NewCollector(),
		EventQueue: make(PriorityQueue, 0),
	}

	s.Nodes = make([]*models.DroneNode, cfg.NumDrones)
	maliciousCount := int(float64(cfg.NumDrones) * cfg.MaliciousRatio)
	for i := 0; i < cfg.NumDrones; i++ {
		isMalicious := i < maliciousCount
		s.Nodes[i] = &models.DroneNode{
			ID:                 i,
			IsMalicious:        isMalicious,
			Location:           models.Point{X: rand.Float64() * cfg.AreaWidth, Y: rand.Float64() * cfg.AreaHeight},
			ComputationalPower: cfg.MinCompPower + rand.Float64()*(cfg.MaxCompPower-cfg.MinCompPower),
			Energy:             cfg.InitialEnergy,
			PacketChannel:      make(chan *models.Packet, 100),
		}
	}

	s.TrustManager = trust.NewManager(s.Nodes, cfg)
	s.ClusterManager = routing.NewClusterManager(s.Nodes, cfg, s.TrustManager)

	// Запускаем обработчики пакетов для каждого дрона в отдельной горутине
	for _, node := range s.Nodes {
		s.Wg.Add(1)
		go s.nodePacketHandler(node)
	}

	return s
}

func (s *Simulator) scheduleEvent(evt *Event) {
	if evt == nil {
		log.Fatalf("FATAL: Attempted to schedule a nil event!")
	}
	s.EventQueueMux.Lock() // <<< ЗАХВАТЫВАЕМ МЬЮТЕКС
	heap.Push(&s.EventQueue, evt)
	s.EventQueueMux.Unlock() // <<< ОСВОБОЖДАЕМ МЬЮТЕКС
}

func (s *Simulator) Run() *metrics.FinalMetrics {
	// log.Println("Начало симуляции...")

	s.scheduleEvent(&Event{Time: 0, Type: EventCHReelection, Data: true})
	for i := range s.Nodes {
		s.scheduleEvent(&Event{Time: rand.Float64(), Type: EventNodeMove, NodeID: i})
		s.scheduleEvent(&Event{Time: 1.0 + rand.Float64(), Type: EventPacketGenerate, NodeID: i})
	}

	for {
		s.EventQueueMux.Lock() // <<< ЗАХВАТЫВАЕМ МЬЮТЕКС ПЕРЕД ПРОВЕРКОЙ И ИЗВЛЕЧЕНИЕМ
		if s.EventQueue.Len() == 0 {
			s.EventQueueMux.Unlock() // <<< ОСВОБОЖДАЕМ, если очередь пуста
			break
		}

		evt := heap.Pop(&s.EventQueue).(*Event)
		s.EventQueueMux.Unlock() // <<< ОСВОБОЖДАЕМ СРАЗУ ПОСЛЕ ИЗВЛЕЧЕНИЯ

		if evt.Time > s.Cfg.SimulationTime {
			break
		}
		s.CurrentTime = evt.Time
		s.handleEvent(evt)
	}

	time.Sleep(100 * time.Millisecond)
	closeAllChannels(s.Nodes)
	s.Wg.Wait()

	// log.Println("Симуляция завершена. Расчет итоговых метрик.")
	return s.Metrics.CalculateFinalMetrics(s)
}

func (s *Simulator) handleEvent(evt *Event) {
	switch evt.Type {
	case EventNodeMove:
		node := s.Nodes[evt.NodeID]
		node.Mutex.Lock()
		node.Location.X += (rand.Float64() - 0.5) * 10
		node.Location.Y += (rand.Float64() - 0.5) * 10
		// Ограничение по полю
		node.Location.X = models.Clamp(node.Location.X, 0, s.Cfg.AreaWidth)
		node.Location.Y = models.Clamp(node.Location.Y, 0, s.Cfg.AreaHeight)
		node.Mutex.Unlock()
		s.scheduleEvent(&Event{Time: s.CurrentTime + 1.0, Type: EventNodeMove, NodeID: evt.NodeID})

	case EventPacketGenerate:
		node := s.Nodes[evt.NodeID]
		if node.IsClusterHead { // CH не генерируют пользовательский трафик
			s.scheduleEvent(&Event{Time: s.CurrentTime + s.Cfg.PacketGenInterval, Type: EventPacketGenerate, NodeID: evt.NodeID})
			return
		}
		destID := rand.Intn(s.Cfg.NumDrones)
		for destID == node.ID {
			destID = rand.Intn(s.Cfg.NumDrones)
		}

		s.PacketCounter++
		packet := &models.Packet{
			ID:            s.PacketCounter,
			SourceID:      node.ID,
			DestinationID: destID,
			CreationTime:  s.CurrentTime,
		}
		s.Metrics.RecordPacketSent()
		node.Mutex.Lock()
		node.PacketsSent++
		node.Mutex.Unlock()

		// Отправляем пакет "в эфир" (в канал обработчика)
		s.routePacket(node, packet)
		s.scheduleEvent(&Event{Time: s.CurrentTime + s.Cfg.PacketGenInterval, Type: EventPacketGenerate, NodeID: evt.NodeID})

	case EventPacketArrival:
		// Это событие теперь обрабатывается в nodePacketHandler
		// Пакет просто отправляется в канал нужного узла
		arrivalEventData := evt.Data.(PacketArrivalData)
		s.Nodes[arrivalEventData.NodeID].PacketChannel <- arrivalEventData.Packet

	case EventCHReelection:
		isInitial := evt.Data.(bool)
		// log.Printf("t=%.2f: Переизбрание Глав Кластеров (CH)...", s.CurrentTime)
		s.ClusterManager.ReelectClusterHeads(s.CurrentTime, s.Metrics)

		// Если это не первые выборы и алгоритм - блокчейн, запускаем консенсус
		if !isInitial && s.Cfg.CHSelectionAlgorithm == "Blockchain" {
			for clusterID := range s.ClusterManager.GetClusters() {
				s.scheduleEvent(&Event{Time: s.CurrentTime, Type: EventConsensusStart, Data: clusterID})
			}
		}

		s.scheduleEvent(&Event{Time: s.CurrentTime + s.Cfg.CHReelectionInterval, Type: EventCHReelection, Data: false})

	case EventConsensusStart:
		clusterID := evt.Data.(int)
		s.Wg.Add(1)
		go consensus.RunConsensusRound(s.CurrentTime, clusterID, s, &s.Wg)

	case EventConsensusEnd:
		// Можно добавить логику обработки результатов консенсуса
	}
}

type PacketArrivalData struct {
	NodeID int
	Packet *models.Packet
}

// nodePacketHandler - горутина, которая обрабатывает пакеты для одного узла
func (s *Simulator) nodePacketHandler(node *models.DroneNode) {
	defer s.Wg.Done()
	for packet := range node.PacketChannel {
		node.Mutex.Lock()
		node.Energy -= s.Cfg.EnergyRx
		node.Mutex.Unlock()

		if node.IsMalicious && rand.Float64() < 0.7 {
			// <<< ИЗМЕНЕНО: Передаем конкретную причину >>>
			s.TrustManager.RecordInteraction(packet.SourceID, node.ID, models.Failure_MaliciousDrop, s.CurrentTime)
			node.Mutex.Lock()
			node.PacketsDroppedByMe++
			node.Mutex.Unlock()
			continue
		}

		if packet.DestinationID == node.ID {
			s.Metrics.RecordPacketDelivered(s.CurrentTime - packet.CreationTime)
			s.Nodes[packet.SourceID].Mutex.Lock()
			s.Nodes[packet.SourceID].PacketsDelivered++
			s.Nodes[packet.SourceID].Mutex.Unlock()
			// <<< ИЗМЕНЕНО: Успешное взаимодействие (условно от лица получателя к отправителю) >>>
			// В текущей модели мы оцениваем только пересылающие узлы, поэтому этот вызов можно убрать
			// s.TrustManager.RecordInteraction(packet.SourceID, node.ID, models.InteractionSuccess, s.CurrentTime)
		} else {
			node.Mutex.Lock()
			node.PacketsForwarded++
			node.Mutex.Unlock()
			s.routePacket(node, packet)
		}
	}
}

// routePacket отправляет пакет следующему узлу или напрямую, если в радиусе
func (s *Simulator) routePacket(sender *models.DroneNode, packet *models.Packet) {
	sender.Mutex.Lock()
	sender.Energy -= s.Cfg.EnergyTx
	sender.Mutex.Unlock()

	packet.Hops++
	if packet.Hops > 15 {
		return
	}

	destinationNode := s.Nodes[packet.DestinationID]

	// Шаг 1: Прямая доставка, если возможно. Это самый высокий приоритет.
	if sender.Location.Distance(destinationNode.Location) <= s.Cfg.CommunicationRadius {
		// Записываем успешное взаимодействие с конечным узлом, если он не отправитель
		// Это поможет поддерживать доверие в сети
		if sender.ID != packet.SourceID {
			s.TrustManager.RecordInteraction(sender.ID, destinationNode.ID, models.InteractionSuccess, s.CurrentTime)
		}
		s.sendPacketToNextHop(sender, destinationNode, packet)
		return
	}

	// Шаг 2: Определяем, является ли маршрутизация внутри- или межкластерной
	senderClusterID := sender.ClusterID
	destClusterID := destinationNode.ClusterID

	var routingTarget *models.DroneNode // Точка, к которой мы стремимся на этом шаге
	isInterCluster := false

	if senderClusterID != -1 && senderClusterID == destClusterID {
		// --- ВНУТРИКЛАСТЕРНАЯ МАРШРУТИЗАЦИЯ ---
		routingTarget = destinationNode
	} else {
		// --- МЕЖКЛАСТЕРНАЯ МАРШРУТИЗАЦИЯ ---
		isInterCluster = true
		destCH := s.ClusterManager.GetNodeClusterHead(destinationNode.ID)
		if destCH == nil {
			// Если у цели нет CH, пытаемся идти напрямую к цели
			routingTarget = destinationNode
		} else {
			routingTarget = destCH
		}
	}

	// Шаг 3: Жадный поиск лучшего следующего узла, который ближе к routingTarget
	var bestNextHop *models.DroneNode
	minDistToTarget := sender.Location.Distance(routingTarget.Location)

	for _, potentialHop := range s.Nodes {
		if potentialHop.ID == sender.ID || potentialHop.ID == destinationNode.ID {
			continue
		}

		if sender.Location.Distance(potentialHop.Location) > s.Cfg.CommunicationRadius {
			continue
		}

		if s.TrustManager.GetTrust(sender.ID, potentialHop.ID) < s.Cfg.TrustThreshold {
			continue
		}

		distFromHopToTarget := potentialHop.Location.Distance(routingTarget.Location)
		if distFromHopToTarget < minDistToTarget {
			minDistToTarget = distFromHopToTarget
			bestNextHop = potentialHop
		}
	}

	// Шаг 4: Отправка
	if bestNextHop != nil {
		s.TrustManager.RecordInteraction(sender.ID, bestNextHop.ID, models.InteractionSuccess, s.CurrentTime)
		s.sendPacketToNextHop(sender, bestNextHop, packet)
	} else if isInterCluster {
		// Если не нашли "транзитный" узел, попробуем отправить напрямую Главе своего кластера в надежде, что он знает путь
		senderCH := s.ClusterManager.GetNodeClusterHead(sender.ID)
		if senderCH != nil && senderCH.ID != sender.ID {
			s.TrustManager.RecordInteraction(sender.ID, senderCH.ID, models.InteractionSuccess, s.CurrentTime)
			s.sendPacketToNextHop(sender, senderCH, packet)
		}
	}
	// Если ни один из вариантов не сработал, пакет теряется.
}

// sendPacketToNextHop - вспомогательная функция для отправки пакета
func (s *Simulator) sendPacketToNextHop(sender, receiver *models.DroneNode, packet *models.Packet) {
	distance := sender.Location.Distance(receiver.Location)

	if distance > s.Cfg.CommunicationRadius {
		// <<< ИЗМЕНЕНО: Записываем потерю из-за разрыва связи >>>
		s.TrustManager.RecordInteraction(sender.ID, receiver.ID, models.Failure_OutOfRange, s.CurrentTime)
		return
	}

	// Эмулируем задержку передачи
	delay := 0.01 + distance/300000000 // Базовая + расстояние/скорость_света (более реалистично)

	s.scheduleEvent(&Event{
		Time: s.CurrentTime + delay,
		Type: EventPacketArrival,
		Data: PacketArrivalData{NodeID: receiver.ID, Packet: packet},
	})
}

func closeAllChannels(nodes []*models.DroneNode) {
	for _, n := range nodes {
		if n.PacketChannel != nil {
			// Безопасное закрытие канала
			func() {
				defer func() {
					if recover() != nil {
						// Канал уже закрыт, ничего не делаем
					}
				}()
				close(n.PacketChannel)
			}()
		}
	}
}

func (s *Simulator) GetClusterHead(clusterID int) *models.DroneNode {
	return s.ClusterManager.GetClusterHead(clusterID)
}
func (s *Simulator) GetClusterMembers(clusterID int) []*models.DroneNode {
	return s.ClusterManager.GetClusterMembers(clusterID)
}
func (s *Simulator) GetConfig() *config.SimulatorConfig {
	return s.Cfg
}
func (s *Simulator) GetNextPacketID() int {
	s.PacketCounter++
	return s.PacketCounter
}

func (s *Simulator) GetTrustManager() consensus.TrustManagerProvider {
	return s.TrustManager
}

func (s *Simulator) GetNodes() []*models.DroneNode {
	return s.Nodes
}
func (s *Simulator) GetTrustManagerForMetrics() metrics.TrustManagerReader {
	return s.TrustManager
}
func (s *Simulator) GetSimulationTime() float64 {
	return s.CurrentTime
}
