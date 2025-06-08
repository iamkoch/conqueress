package conqueress

import "testing"

type IsEvent struct {
	*BaseEvent
	Name string
}

func TestNewEventCreation(t *testing.T) {
	event := NewEvent[IsEvent]()
	event.Name = "test"

	if event.MessageId == "" {
		t.Error("Message id should not be empty")
	}

	if event.Ver != -1 {
		t.Error("event.Ver should be -1")
	}
}

func TestNewEventCreationWithModifiers(t *testing.T) {
	event := NewEvent[IsEvent](func(e *IsEvent) {
		e.Name = "test22"
	})

	if event.MessageId == "" {
		t.Error("Message id should not be empty")
	}

	if event.Ver != -1 {
		t.Error("event.Ver should be -1")
	}

	if event.Name != "test22" {
		t.Error("name should be set to test22")
	}
}

//
//func TestNewPointerEventCreation(t *testing.T) {
//	event := NewEvent[*IsEvent]()
//	event.Name = "test"
//
//	if event.MessageId == "" {
//		t.Error("Message id should not be empty")
//	}
//
//	if event.Ver != -1 {
//		t.Error("event.Ver should be -1")
//	}
//}
