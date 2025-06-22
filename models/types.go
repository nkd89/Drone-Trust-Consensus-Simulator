package models

import (
	"fmt"
	"math"
	"sync"
)

type Point struct {
	X, Y float64
}

func (p Point) Distance(other Point) float64 {
	return math.Sqrt(math.Pow(p.X-other.X, 2) + math.Pow(p.Y-other.Y, 2))
}

type DroneNode struct {
	ID                 int
	IsMalicious        bool
	Location           Point
	ComputationalPower float64 // Вычислительная мощность в Gflops
	Mutex              sync.RWMutex

	// Состояние
	IsClusterHead bool
	ClusterID     int
	Energy        float64

	// Статистика для PoRS и метрик
	PacketsSent         int
	PacketsDelivered    int
	PacketsForwarded    int
	PacketsDroppedByMe  int
	ConsensusRounds     int // Участие в консенсусе (для фактора RF)
	ValidBlocksProposed int // Валидные блоки (для фактора RF)

	// Канал для получения пакетов (для эмуляции)
	PacketChannel chan *Packet
}

type Packet struct {
	ID            int
	SourceID      int
	DestinationID int
	CreationTime  float64
	Hops          int
	IsAck         bool // Является ли пакет подтверждением
}

type Block struct {
	ID         int
	ProposerID int
	Timestamp  float64
	// Здесь можно добавить транзакции, хэши и т.д. для более полной эмуляции
}

func (n *DroneNode) String() string {
	return fmt.Sprintf("Drone %d", n.ID)
}

func Clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

type InteractionResult int

const (
	InteractionSuccess    InteractionResult = iota // Успешная доставка/пересылка
	Failure_MaliciousDrop                          // Злонамеренный сброс пакета
	Failure_OutOfRange                             // Потеря из-за разрыва связи
	Failure_NoRoute                                // Не удалось найти следующий узел
	Failure_PacketLoop                             // Превышен лимит хопов
)
