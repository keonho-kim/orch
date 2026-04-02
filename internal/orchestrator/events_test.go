package orchestrator

import "testing"

func TestSubscribeEventsBroadcastsToMultipleSubscribers(t *testing.T) {
	t.Parallel()

	service := &Service{}
	first, cancelFirst := service.SubscribeEvents()
	defer cancelFirst()
	second, cancelSecond := service.SubscribeEvents()
	defer cancelSecond()

	service.EmitEvent(ServiceEvent{Type: "snapshot", Message: "hello"})

	select {
	case event := <-first:
		if event.Type != "snapshot" || event.Message != "hello" {
			t.Fatalf("unexpected first event: %+v", event)
		}
	default:
		t.Fatal("expected first subscriber to receive event")
	}

	select {
	case event := <-second:
		if event.Type != "snapshot" || event.Message != "hello" {
			t.Fatalf("unexpected second event: %+v", event)
		}
	default:
		t.Fatal("expected second subscriber to receive event")
	}
}
