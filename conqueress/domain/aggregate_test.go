package domain

import (
	"testing"

	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/guid"
	"github.com/stretchr/testify/assert"
)

// TestEvent is a simple event implementation for testing
type TestEvent struct {
	*cqrs.BaseEvent
	EventData string
}

// TestAggregate is a simple aggregate implementation for testing
type TestAggregate struct {
	Base      AggregateRootBase[guid.Guid]
	TestValue string
}

func (a *TestAggregate) SetBase(base AggregateRootBase[guid.Guid]) {
	a.Base = base
}

func (a *TestAggregate) GetHandler() func(e cqrs.Event) {
	return func(e cqrs.Event) {
		if testEvent, ok := e.(TestEvent); ok {
			a.TestValue = testEvent.EventData
		}
	}
}

func (a *TestAggregate) SetInnerApply(ia func(e cqrs.Event)) {
	a.Base.SetInnerApply(ia)
}

func (a *TestAggregate) Id() guid.Guid {
	return a.Base.Id()
}

func (a *TestAggregate) UncommittedEvents() []cqrs.Event {
	return a.Base.UncommittedEvents()
}

func (a *TestAggregate) ApplyChange(e cqrs.Event) {
	a.Base.ApplyChange(e)
}

// TestAggregateWithCustomID is an aggregate with a custom ID type
type TestAggregateWithCustomID struct {
	Base      AggregateRootBase[string]
	TestValue string
}

func (a *TestAggregateWithCustomID) SetBase(base AggregateRootBase[string]) {
	a.Base = base
}

func (a *TestAggregateWithCustomID) GetHandler() func(e cqrs.Event) {
	return func(e cqrs.Event) {
		if testEvent, ok := e.(TestEvent); ok {
			a.TestValue = testEvent.EventData
		}
	}
}

func (a *TestAggregateWithCustomID) SetInnerApply(ia func(e cqrs.Event)) {
	a.Base.SetInnerApply(ia)
}

func (a *TestAggregateWithCustomID) Id() string {
	return a.Base.Id()
}

func (a *TestAggregateWithCustomID) UncommittedEvents() []cqrs.Event {
	return a.Base.UncommittedEvents()
}

func (a *TestAggregateWithCustomID) ApplyChange(e cqrs.Event) {
	a.Base.ApplyChange(e)
}

func TestAggregateRootBase_SetId(t *testing.T) {
	// Arrange
	base := NewAggregate[guid.Guid]()
	id := guid.New()

	// Act
	base.SetId(id)

	// Assert
	assert.Equal(t, id, base.Id())
}

func TestAggregateRootBase_SetVersion(t *testing.T) {
	// Arrange
	base := NewAggregate[guid.Guid]()
	version := 42

	// Act
	base.SetVersion(version)

	// Assert
	assert.Equal(t, version, base._version)
}

func TestAggregateRootBase_ApplyChange(t *testing.T) {
	// Arrange
	base := NewAggregate[guid.Guid]()
	handlerCalled := false
	base.SetInnerApply(func(e cqrs.Event) {
		handlerCalled = true
	})

	event := cqrs.NewEvent[TestEvent](func(e *TestEvent) {
		e.EventData = "test data"
	})

	// Act
	base.ApplyChange(event)

	// Assert
	assert.True(t, handlerCalled, "Event handler should be called")
	assert.Len(t, base.UncommittedEvents(), 1, "Should have one uncommitted event")
	assert.Equal(t, event, base.UncommittedEvents()[0], "Uncommitted event should match applied event")
}

func TestNew(t *testing.T) {
	// Act
	aggregate := New[TestAggregate]()

	// Assert
	assert.NotNil(t, aggregate, "Aggregate should not be nil")
	
	// Apply an event to test the handler
	event := cqrs.NewEvent[TestEvent](func(e *TestEvent) {
		e.EventData = "test data"
	})
	aggregate.ApplyChange(event)
	
	assert.Equal(t, "test data", aggregate.TestValue, "Event handler should update test value")
	assert.Len(t, aggregate.UncommittedEvents(), 1, "Should have one uncommitted event")
}

func TestNewWithID(t *testing.T) {
	// Act
	aggregate := NewWithID[TestAggregateWithCustomID, string]()

	// Assert
	assert.NotNil(t, aggregate, "Aggregate should not be nil")
	
	// Set ID and apply an event
	aggregate.Base.SetId("custom-id")
	event := cqrs.NewEvent[TestEvent](func(e *TestEvent) {
		e.EventData = "test data with custom ID"
	})
	aggregate.ApplyChange(event)
	
	assert.Equal(t, "custom-id", aggregate.Id(), "ID should be set correctly")
	assert.Equal(t, "test data with custom ID", aggregate.TestValue, "Event handler should update test value")
	assert.Len(t, aggregate.UncommittedEvents(), 1, "Should have one uncommitted event")
}

func TestGetDefaultAggregate(t *testing.T) {
	// Act
	aggregate := GetDefaultAggregate[TestAggregate]()

	// Assert
	assert.NotNil(t, aggregate, "Aggregate should not be nil")
	
	// Apply an event to test the handler
	event := cqrs.NewEvent[TestEvent](func(e *TestEvent) {
		e.EventData = "test data from default"
	})
	aggregate.ApplyChange(event)
	
	assert.Equal(t, "test data from default", aggregate.TestValue, "Event handler should update test value")
	assert.Len(t, aggregate.UncommittedEvents(), 1, "Should have one uncommitted event")
} 