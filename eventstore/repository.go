package eventstore

import (
	"errors"
	"github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/domain"
	"github.com/iamkoch/conqueress/guid"
	"reflect"
)

var (
	ErrConcurrencyException = errors.New("concurrency exception")
)

type IEventStore interface {
	SaveEvents(aggregateType string, aggregateId guid.Guid, events []conqueress.Event, expectedVersion int) error
	GetEventsForAggregate(aggregateId guid.Guid) []conqueress.Event
}

type IGenericIDEventStore[TID any] interface {
	SaveEvents(aggregateType string, aggregateId TID, events []conqueress.Event, expectedVersion int) error
	GetEventsForAggregate(aggregateId TID) []conqueress.Event
}

type Repository[T domain.IAggregate] interface {
	GetById(id guid.Guid) (T, error)
	Save(aggregate T, expectedVersion int) error
}

type GenericIDRepository[T domain.IGenericIDAggregate[TID], TID any] interface {
	GetById(id TID) (T, error)
	Save(aggregate T, expectedVersion int) error
}

var (
	ErrAggregateNotFound = errors.New("aggregate not found")
)

type genericRepository[T domain.IAggregate] struct {
	store          IEventStore
	createInstance func() T
}

type genericIDRepository[T domain.IGenericIDAggregate[TID], TID any] struct {
	store          IGenericIDEventStore[TID]
	createInstance func() T
}

func (g genericRepository[T]) GetById(id guid.Guid) (T, error) {
	events := g.store.GetEventsForAggregate(id)
	if len(events) == 0 {
		var t T
		return t, ErrAggregateNotFound
	}
	agg := g.createInstance()
	for _, e := range events {
		reflect.ValueOf(agg).Interface().(domain.InnerApplier).InnerApply(e)
	}
	return agg, nil
}

func (g genericRepository[T]) Save(aggregate T, expectedVersion int) error {
	e := g.store.SaveEvents(
		reflect.TypeOf(aggregate).Name(),
		aggregate.Id(),
		aggregate.UncommittedEvents(),
		expectedVersion)

	return e
}

func (g genericIDRepository[T, TID]) GetById(id TID) (T, error) {
	events := g.store.GetEventsForAggregate(id)
	if len(events) == 0 {
		var t T
		return t, ErrAggregateNotFound
	}
	agg := g.createInstance()
	for _, e := range events {
		reflect.ValueOf(agg).Interface().(domain.InnerApplier).InnerApply(e)
	}
	return agg, nil
}

func (g genericIDRepository[T, TID]) Save(aggregate T, expectedVersion int) error {
	e := g.store.SaveEvents(
		reflect.TypeOf(aggregate).Name(),
		aggregate.Id(),
		aggregate.UncommittedEvents(),
		expectedVersion)

	return e
}

func NewRepository[T domain.IAggregate](
	store IEventStore,
	createInstance func() T) Repository[T] {
	return genericRepository[T]{store, createInstance}
}

func NewGenericIDRepository[T domain.IGenericIDAggregate[TID], TID any](
	store IGenericIDEventStore[TID],
	createInstance func() T) GenericIDRepository[T, TID] {
	return genericIDRepository[T, TID]{store, createInstance}
}
