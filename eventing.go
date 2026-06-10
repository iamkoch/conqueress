// Package conqueress provides primitives for command-query-responsibility-
// separation and event-sourcing in Go.
//
// This file defines the Event, IntegrationEvent, and Command surface.
package conqueress

import (
	"reflect"
	"time"

	"github.com/iamkoch/conqueress/guid"
)

// Event is the contract for a domain event emitted by an aggregate.
//
// Events carry their own identity, ordering version, correlation/causation
// trace and the wall-clock moment at which the domain modelled their
// occurrence. Consumers embed *BaseEvent in their event structs to satisfy
// this interface for free.
type Event interface {
	// MsgId returns the event's stable identity. Distinct from the aggregate
	// version: the event ID identifies one event, not one position in a
	// stream.
	MsgId() guid.Guid

	// Version returns the position of this event in its aggregate's stream
	// (0-based). Set by the event store at append time.
	Version() int

	// WithVersion sets the event's position. Called by the event store.
	WithVersion(v int)

	// CorrelationId returns the trace correlation ID. Propagated from the
	// originating command (or external trigger) through every event and
	// downstream command it causes. Empty if not yet set.
	CorrelationId() guid.Guid

	// CausationId returns the ID of the message that directly caused this
	// event. For an event emitted by a command handler, this is the
	// command's ID. Empty if not yet set.
	CausationId() guid.Guid

	// OccurredAt is the wall-clock moment the domain modelled the event as
	// having occurred. Distinct from "the time the event was persisted" —
	// the domain owns this timestamp.
	OccurredAt() time.Time

	// WithMetadata stamps correlation/causation/occurredAt onto the event,
	// returning the event for fluent chaining. Used by the command handler
	// or framework integration code, not by aggregates themselves.
	WithMetadata(correlationId, causationId guid.Guid, occurredAt time.Time)
}

// IntegrationEvent is a marker interface for events intended to cross
// service boundaries on a message bus. Domain events stay internal to the
// aggregate; integration events are the cross-context contract.
//
// Implementations typically wrap (or translate from) a domain event. The
// integrationevents ACL in a service is responsible for that translation.
type IntegrationEvent interface {
	Event

	// EventType is the wire-stable name used to route the event to consumers
	// in other services (e.g. "com.inshur.policy.created"). Distinct from
	// the Go type name so wire schemas can outlive Go renames.
	EventType() string
}

// BaseEvent is the embeddable base type for events. Consumers embed it as
// *BaseEvent in their event structs:
//
//	type InventoryItemCreated struct {
//	    *conqueress.BaseEvent
//	    Id   guid.Guid
//	    Name string
//	}
type BaseEvent struct {
	MessageId   string    `json:"message_id"`
	Ver         int       `json:"version"`
	Correlation string    `json:"correlation_id,omitempty"`
	Causation   string    `json:"causation_id,omitempty"`
	Occurred    time.Time `json:"occurred_at,omitempty"`
}

// Version returns the event's stream-position.
func (b *BaseEvent) Version() int {
	return b.Ver
}

// MsgId returns the event's stable identity.
func (b *BaseEvent) MsgId() guid.Guid {
	return guid.MustFromString(b.MessageId)
}

// WithVersion sets the event's stream-position. Called by the event store.
func (b *BaseEvent) WithVersion(v int) {
	b.Ver = v
}

// CorrelationId returns the trace correlation ID, or guid.Empty if unset.
func (b *BaseEvent) CorrelationId() guid.Guid {
	if b.Correlation == "" {
		return guid.Empty
	}
	id, err := guid.FromString(b.Correlation)
	if err != nil {
		return guid.Empty
	}
	return id
}

// CausationId returns the ID of the message that caused this event, or
// guid.Empty if unset.
func (b *BaseEvent) CausationId() guid.Guid {
	if b.Causation == "" {
		return guid.Empty
	}
	id, err := guid.FromString(b.Causation)
	if err != nil {
		return guid.Empty
	}
	return id
}

// OccurredAt returns the domain wall-clock time stamped onto the event.
func (b *BaseEvent) OccurredAt() time.Time {
	return b.Occurred
}

// WithMetadata stamps correlation/causation/occurredAt onto the event in
// place. Typically called by the command handler that emitted the event,
// before the repository persists it.
func (b *BaseEvent) WithMetadata(correlationId, causationId guid.Guid, occurredAt time.Time) {
	b.Correlation = correlationId.String()
	b.Causation = causationId.String()
	b.Occurred = occurredAt
}

func defaultBaseEvent() *BaseEvent {
	return &BaseEvent{Ver: -1, MessageId: guid.New().String()}
}

// NewEvent constructs an instance of T and populates its embedded BaseEvent
// field if one is present and settable. T must not be a pointer type.
//
// Optional setters can supply field values inline:
//
//	created := conqueress.NewEvent[InventoryItemCreated](func(e *InventoryItemCreated) {
//	    e.Id = id
//	    e.Name = name
//	})
func NewEvent[T any](setters ...func(*T)) T {
	var t T

	tType := reflect.TypeOf(t)
	if tType.Kind() == reflect.Ptr {
		panic("T cannot be a pointer")
	}

	tValue := reflect.New(tType).Elem()
	baseEvent := defaultBaseEvent()

	for i := 0; i < tValue.NumField(); i++ {
		field := tValue.Type().Field(i)
		if field.Name == "BaseEvent" {
			fieldValue := tValue.FieldByName("BaseEvent")
			if fieldValue.CanSet() {
				if field.Type.Kind() == reflect.Ptr {
					fieldValue.Set(reflect.ValueOf(baseEvent))
				} else {
					fieldValue.Set(reflect.ValueOf(*baseEvent))
				}
			}
		}
	}

	if len(setters) == 0 {
		return tValue.Interface().(T)
	}

	outputValue := tValue.Interface().(T)
	for _, setter := range setters {
		setter(&outputValue)
	}

	return outputValue
}

// Command is the marker interface for commands accepted by a Mediator.
// Plain structs satisfy it without any embedded base; embed *BaseCommand
// if you want correlation/causation/createdAt for free.
type Command interface{}

// BaseCommand is the embeddable base for a command that needs to carry
// correlation/causation through the framework.
type BaseCommand struct {
	CommandId   string    `json:"command_id"`
	Correlation string    `json:"correlation_id"`
	Causation   string    `json:"causation_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewBaseCommand constructs a BaseCommand with a fresh ID and now() timestamp.
// The caller supplies correlation and causation; for an externally-triggered
// command (e.g. from HTTP), set correlation to a fresh ID and causation to
// the same value (or to the upstream trace ID).
func NewBaseCommand(correlation, causation guid.Guid) BaseCommand {
	return BaseCommand{
		CommandId:   guid.New().String(),
		Correlation: correlation.String(),
		Causation:   causation.String(),
		CreatedAt:   time.Now().UTC(),
	}
}

// Id returns the command's stable identity.
func (c BaseCommand) Id() guid.Guid {
	return guid.MustFromString(c.CommandId)
}

// CorrelationId returns the command's trace correlation ID.
func (c BaseCommand) CorrelationId() guid.Guid {
	return guid.MustFromString(c.Correlation)
}

// CausationId returns the ID of the message that caused this command.
func (c BaseCommand) CausationId() guid.Guid {
	return guid.MustFromString(c.Causation)
}
