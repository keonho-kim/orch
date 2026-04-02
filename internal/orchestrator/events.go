package orchestrator

type ServiceEvent struct {
	Type      string         `json:"type"`
	SessionID string         `json:"session_id,omitempty"`
	RunID     string         `json:"run_id,omitempty"`
	Message   string         `json:"message,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type UIEvent = ServiceEvent

func (s *Service) SubscribeEvents() (<-chan ServiceEvent, func()) {
	ch := make(chan ServiceEvent, 256)

	s.eventMu.Lock()
	if s.subscribers == nil {
		s.subscribers = make(map[int]chan ServiceEvent)
	}
	id := s.nextSubscriberID
	s.nextSubscriberID++
	s.subscribers[id] = ch
	s.eventMu.Unlock()

	cancel := func() {
		s.eventMu.Lock()
		subscriber, ok := s.subscribers[id]
		if ok {
			delete(s.subscribers, id)
			close(subscriber)
		}
		s.eventMu.Unlock()
	}

	return ch, cancel
}

func (s *Service) EmitEvent(event ServiceEvent) {
	s.publish(event)
}

func (s *Service) publish(event ServiceEvent) {
	s.eventMu.RLock()
	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	s.eventMu.RUnlock()
}
