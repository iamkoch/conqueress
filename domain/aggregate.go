package domain

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/guid"
	"reflect"
)

type IAggregate interface {
	Id() guid.Guid
	UncommittedEvents() []cqrs.Event
	ApplyChange(e cqrs.Event)
}

type IGenericIDAggregate[TID any] interface {
	Id() TID
	UncommittedEvents() []cqrs.Event
	ApplyChange(e cqrs.Event)
}

type AggregateRootBase[TID any] struct {
	_changes    []cqrs.Event
	_id         TID
	_version    int
	_innerApply func(e cqrs.Event)
}

func (a *AggregateRootBase[TID]) SetId(id TID) {
	a._id = id
}

func (a *AggregateRootBase[TID]) SetVersion(v int) {
	a._version = v
}

func (a *AggregateRootBase[TID]) SetInnerApply(ia func(e cqrs.Event)) {
	a._innerApply = ia
}

func (a *AggregateRootBase[TID]) InnerApply(e cqrs.Event) {
	a._innerApply(e)
}

// Replay applies a stored event, moving the aggregate to that event's version.
// The event is not recorded as an uncommitted change, because it is already in
// the store.
func (a *AggregateRootBase[TID]) Replay(e cqrs.Event) {
	a.applyChangeInternal(e, false)
}

// Replayer is how a repository rebuilds an aggregate from its stored events.
type Replayer interface {
	Replay(e cqrs.Event)
}

type DefaultAggregate[TID any] interface {
	SetBase(base AggregateRootBase[TID])
	GetHandler() func(e cqrs.Event)
	SetInnerApply(ia func(e cqrs.Event))
}

func (a *AggregateRootBase[TID]) Id() TID {
	return a._id
}

// Version returns the version of the last stored event applied to the
// aggregate. An aggregate that has never been saved is at -1, and one loaded
// from an event store is at the version of its most recent stored event.
// Applying new changes does not move it, because those events have no version
// until they are saved.
//
// Pass this to Repository.Save as the expected version to make the write
// conditional on nothing else having written to the stream in the meantime.
func (a *AggregateRootBase[TID]) Version() int {
	return a._version
}

func (a *AggregateRootBase[TID]) applyChangeInternal(e cqrs.Event, isNew bool) {
	loaded := a._version

	a.InnerApply(e)

	if isNew {
		// A new event has no version until it is saved, so the aggregate stays
		// at the version it loaded with. A handler that restores the version
		// from the event would otherwise reset it to -1 here, and the next
		// save would look like the first write to the stream and skip the
		// concurrency check.
		a._version = loaded
		a._changes = append(a._changes, e)
		return
	}

	// A replayed event carries the version it was stored at.
	a._version = e.Version()
}

func (a *AggregateRootBase[TID]) ApplyChange(e cqrs.Event) {
	a.applyChangeInternal(e, true)
}

func (a *AggregateRootBase[TID]) UncommittedEvents() []cqrs.Event {
	return a._changes
}

func NewAggregate[TID any]() AggregateRootBase[TID] {
	return AggregateRootBase[TID]{}
}

func New[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[guid.Guid])

	a.SetBase(NewAggregate[guid.Guid]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

func NewWithID[T any, TID any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[TID])

	a.SetBase(NewAggregate[TID]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

func GetDefaultAggregate[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[guid.Guid])

	a.SetBase(NewAggregate[guid.Guid]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}
