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

type Repository[T domain.IAggregate] interface {
	GetById(id guid.Guid) T
	Save(aggregate T, expectedVersion int) error
}

type genericRepository[T domain.IAggregate] struct {
	store          IEventStore
	createInstance func() T
}

func (g genericRepository[T]) GetById(id guid.Guid) T {
	events := g.store.GetEventsForAggregate(id)
	agg := g.createInstance()
	for _, e := range events {
		reflect.ValueOf(agg).Interface().(domain.InnerApplier).InnerApply(e)
	}
	return agg
}

func (g genericRepository[T]) Save(aggregate T, expectedVersion int) error {
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
