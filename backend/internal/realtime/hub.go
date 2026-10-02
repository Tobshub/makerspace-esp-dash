package realtime

import "sync"

// Hub delivers events to the subscribers of one project.
// A slow client drops events instead of blocking ingestion.
type Hub struct {
	mu   sync.Mutex
	next int
	subs map[string]map[int]chan Event
}

func New() *Hub {
	return &Hub{subs: map[string]map[int]chan Event{}}
}

func (h *Hub) Publish(projectID string, event Event) {
	if h == nil || projectID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs[projectID] {
		select {
		case ch <- event:
		default:
		}
	}
}

// Subscribe returns events for projectID. Cancel removes the subscription.
func (h *Hub) Subscribe(projectID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	if h == nil {
		close(ch)
		return ch, func() {}
	}
	h.mu.Lock()
	id := h.next
	h.next++
	if h.subs[projectID] == nil {
		h.subs[projectID] = map[int]chan Event{}
	}
	h.subs[projectID][id] = ch
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		subs := h.subs[projectID]
		if subs == nil {
			return
		}
		current, ok := subs[id]
		if !ok {
			return
		}
		delete(subs, id)
		close(current)
		if len(subs) == 0 {
			delete(h.subs, projectID)
		}
	}
	return ch, cancel
}
