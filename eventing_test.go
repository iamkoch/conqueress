package conqueress

import (
	"testing"
	"time"

	"github.com/iamkoch/conqueress/guid"
	"github.com/stretchr/testify/assert"
)

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

func TestBaseEvent_WithMetadataPopulatesCorrelationCausationAndOccurredAt(t *testing.T) {
	correlation := guid.New()
	causation := guid.New()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	evt := NewEvent[IsEvent](func(e *IsEvent) {
		e.Name = "widget"
	})
	evt.WithMetadata(correlation, causation, now)

	assert.Equal(t, correlation, evt.CorrelationId())
	assert.Equal(t, causation, evt.CausationId())
	assert.Equal(t, now, evt.OccurredAt())
	assert.Equal(t, "widget", evt.Name)
}

func TestBaseEvent_DefaultsReturnEmptyGuidsAndZeroTime(t *testing.T) {
	evt := NewEvent[IsEvent]()

	assert.Equal(t, guid.Empty, evt.CorrelationId())
	assert.Equal(t, guid.Empty, evt.CausationId())
	assert.True(t, evt.OccurredAt().IsZero())
}

func TestNewBaseCommand_PopulatesIdAndCorrelationAndCausation(t *testing.T) {
	correlation := guid.New()
	causation := guid.New()

	cmd := NewBaseCommand(correlation, causation)

	assert.NotEqual(t, guid.Empty, cmd.Id())
	assert.Equal(t, correlation, cmd.CorrelationId())
	assert.Equal(t, causation, cmd.CausationId())
	assert.False(t, cmd.CreatedAt.IsZero())
}
