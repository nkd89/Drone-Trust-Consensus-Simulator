package simulator

import (
	"log"
)

type EventType int

const (
	EventNodeMove EventType = iota
	EventPacketGenerate
	EventPacketArrival
	EventCHReelection
	EventConsensusStart
	EventConsensusEnd
)

type Event struct {
	Time   float64
	Type   EventType
	NodeID int
	Data   interface{} // Дополнительные данные, например, сам пакет
	index  int
}

// PriorityQueue реализует heap.Interface
type PriorityQueue []*Event

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// <<< Добавляем проверку на nil >>>
	if pq[i] == nil {
		log.Fatalf("FATAL: pq[i] is nil at index %d", i)
	}
	if pq[j] == nil {
		log.Fatalf("FATAL: pq[j] is nil at index %d", j)
	}
	return pq[i].Time < pq[j].Time
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	event, ok := x.(*Event)
	if !ok {
		log.Fatalf("FATAL: Pushed a non-event type into queue. Type: %T", x)
	}
	if event == nil {
		log.Fatalf("FATAL: Pushed a nil event into queue.")
	}

	n := len(*pq)
	event.index = n
	*pq = append(*pq, event)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	event := old[n-1]

	// old[n-1] = nil // <<< ВРЕМЕННО КОММЕНТИРУЕМ ЭТУ СТРОКУ

	// Добавим проверку, чтобы убедиться
	if event == nil {
		log.Fatalf("FATAL: event is nil just before modifying index. Queue length: %d", n)
	}

	event.index = -1
	*pq = old[0 : n-1]
	return event
}
