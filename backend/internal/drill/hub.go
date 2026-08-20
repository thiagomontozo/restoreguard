package drill

import (
	"encoding/json"
	"sync"
)

type Event struct {
	DrillID string `json:"drillId"`
	Type    string `json:"type"`
	Data    any    `json:"data"`
}
type EventHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan []byte]struct{}
}

func NewEventHub() *EventHub { return &EventHub{subscribers: map[string]map[chan []byte]struct{}{}} }
func (h *EventHub) Subscribe(drillID string) (<-chan []byte, func()) {
	channel := make(chan []byte, 32)
	h.mu.Lock()
	if h.subscribers[drillID] == nil {
		h.subscribers[drillID] = map[chan []byte]struct{}{}
	}
	h.subscribers[drillID][channel] = struct{}{}
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		if _, ok := h.subscribers[drillID][channel]; ok {
			delete(h.subscribers[drillID], channel)
			close(channel)
		}
		h.mu.Unlock()
	}
}
func (h *EventHub) Publish(event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for channel := range h.subscribers[event.DrillID] {
		select {
		case channel <- payload:
		default:
		}
	}
}
func (h *EventHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, channels := range h.subscribers {
		for channel := range channels {
			close(channel)
		}
		delete(h.subscribers, id)
	}
}
