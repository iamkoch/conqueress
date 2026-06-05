// Package domain defines the aggregate-root primitives that consumers embed in
// their own domain types.
//
// The pattern is type-switch dispatch with an injected handler:
//
//  1. The consumer's aggregate (e.g. `*InventoryItem`) embeds
//     `AggregateRootBase[TID]`.
//  2. The consumer defines a single `handleEvent(e cqrs.Event)` method on the
//     aggregate that uses a type switch over the event types it handles.
//  3. At construction, the consumer calls `SetInnerApply(handleEvent)` to
//     install that callback onto the base.
//  4. From then on, `ApplyChange(evt)` both appends to the uncommitted-changes
//     list and dispatches to the consumer's type switch.
//
// No reflection, no dynamic dispatch. The consumer owns the dispatch table as
// a normal `switch evt := e.(type)`.
package domain

import (
	"reflect"

	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/guid"
)

// IAggregate is the legacy non-generic interface backed by guid.Guid IDs.
// Kept for backwards compatibility; prefer IGenericIDAggregate[TID] for new code.
type IAggregate interface {
	Id() guid.Guid
	UncommittedEvents() []cqrs.Event
	ApplyChange(e cqrs.Event)
}

// IGenericIDAggregate is the aggregate contract parameterised by ID type.
// Use this when your aggregate uses a typed ID other than guid.Guid (e.g. a
// composite key value type).
type IGenericIDAggregate[TID any] interface {
	Id() TID
	UncommittedEvents() []cqrs.Event
	ApplyChange(e cqrs.Event)
}

// AggregateRootBase is the embeddable base for an event-sourced aggregate.
//
// Consumers embed it in their aggregate struct and inject a `handleEvent`
// callback via SetInnerApply during construction. ApplyChange both routes the
// event through that callback (so the aggregate's state is updated) and
// appends to the uncommitted-changes list (so the repository can persist).
type AggregateRootBase[TID any] struct {
	changes    []cqrs.Event
	id         TID
	version    int
	innerApply func(e cqrs.Event)
}

// SetId sets the aggregate's identity. Typically called from the consumer's
// `handleEvent` when handling the "created" event for the aggregate.
func (a *AggregateRootBase[TID]) SetId(id TID) {
	a.id = id
}

// SetVersion sets the aggregate's version, typically set on each handled event.
func (a *AggregateRootBase[TID]) SetVersion(v int) {
	a.version = v
}

// Version returns the aggregate's current version.
func (a *AggregateRootBase[TID]) Version() int {
	return a.version
}

// SetInnerApply installs the consumer's event-handling callback. The consumer
// typically passes a method value like `aggregate.handleEvent` here.
func (a *AggregateRootBase[TID]) SetInnerApply(ia func(e cqrs.Event)) {
	a.innerApply = ia
}

// InnerApply invokes the consumer-supplied callback. Used internally by
// ApplyChange and by the repository when rehydrating from events.
func (a *AggregateRootBase[TID]) InnerApply(e cqrs.Event) {
	a.innerApply(e)
}

// InnerApplier exposes the InnerApply method so the repository can route
// historical events through the consumer's handler without exposing the base
// struct's internals.
type InnerApplier interface {
	InnerApply(e cqrs.Event)
}

// DefaultAggregate is the interface a consumer's aggregate must satisfy if
// it's going to be constructed via the reflection-based helpers (New[T],
// NewWithID[T,TID], GetDefaultAggregate[T]).
//
// If you don't use those helpers, you don't need to satisfy this interface;
// you can construct your aggregate directly via composite literal and call
// SetInnerApply yourself. That's the recommended pattern for new code — see
// the package doc.
type DefaultAggregate[TID any] interface {
	SetBase(base AggregateRootBase[TID])
	GetHandler() func(e cqrs.Event)
	SetInnerApply(ia func(e cqrs.Event))
}

// Id returns the aggregate's identity.
func (a *AggregateRootBase[TID]) Id() TID {
	return a.id
}

func (a *AggregateRootBase[TID]) applyChangeInternal(e cqrs.Event, isNew bool) {
	a.InnerApply(e)

	if isNew {
		a.changes = append(a.changes, e)
	}
}

// ApplyChange routes the event through the consumer's handler and records it
// as an uncommitted change. Used by the consumer's aggregate methods (e.g.
// `(i *InventoryItem) Rename(...)`).
func (a *AggregateRootBase[TID]) ApplyChange(e cqrs.Event) {
	a.applyChangeInternal(e, true)
}

// UncommittedEvents returns the events emitted since the aggregate was loaded
// or last persisted. The repository drains this list at Save time.
func (a *AggregateRootBase[TID]) UncommittedEvents() []cqrs.Event {
	return a.changes
}

// MarkChangesAsCommitted clears the uncommitted-changes buffer. The repository
// calls this after a successful Save so subsequent calls only return new events.
func (a *AggregateRootBase[TID]) MarkChangesAsCommitted() {
	a.changes = a.changes[:0]
}

// NewAggregate returns a zero-value AggregateRootBase. Consumers use it when
// constructing their aggregate via composite literal.
func NewAggregate[TID any]() AggregateRootBase[TID] {
	return AggregateRootBase[TID]{}
}

// New constructs an aggregate of type T via reflection, defaulting its ID
// type to guid.Guid. The aggregate must satisfy DefaultAggregate[guid.Guid].
//
// Prefer constructing aggregates explicitly via composite literal + manual
// SetInnerApply call; this helper exists for symmetry with the C# template
// and may be removed in a future major version.
func New[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[guid.Guid])

	a.SetBase(NewAggregate[guid.Guid]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

// NewWithID is the typed-ID variant of New.
func NewWithID[T any, TID any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[TID])

	a.SetBase(NewAggregate[TID]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

// GetDefaultAggregate is the constructor factory function the repository can
// use to create a fresh instance during event rehydration. Behaves like
// New[T] but returns a function-callable form.
func GetDefaultAggregate[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[guid.Guid])

	a.SetBase(NewAggregate[guid.Guid]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}
