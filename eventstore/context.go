package eventstore

import (
	"context"
	"reflect"

	"github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/domain"
)

// Store is the context-aware persistence port for an event stream, generic
// over the aggregate ID type. Production implementations (MongoDB, Postgres,
// etc.) should implement this interface rather than the legacy IEventStore /
// IGenericIDEventStore, which lack context propagation and error returns on
// reads.
//
// SaveEvents must assign stream positions to the events (via WithVersion),
// starting from expectedVersion+1, and must fail with a concurrency error if
// the stream's current version does not equal expectedVersion (use -1 for a
// new stream).
type Store[TID any] interface {
	SaveEvents(ctx context.Context, aggregateType string, aggregateId TID, events []conqueress.Event, expectedVersion int) error
	GetEventsForAggregate(ctx context.Context, aggregateId TID) ([]conqueress.Event, error)
}

// ContextRepository loads and saves aggregates against a Store. The
// context-aware counterpart of GenericIDRepository.
type ContextRepository[T domain.IGenericIDAggregate[TID], TID any] interface {
	// GetById rehydrates the aggregate from its event stream. Returns
	// ErrAggregateNotFound when the stream is empty.
	GetById(ctx context.Context, id TID) (T, error)

	// Save appends the aggregate's uncommitted events at expectedVersion and,
	// on success, marks them committed so subsequent saves only append new
	// events.
	Save(ctx context.Context, aggregate T, expectedVersion int) error
}

type contextRepository[T domain.IGenericIDAggregate[TID], TID any] struct {
	store          Store[TID]
	createInstance func() T
}

// NewContextRepository builds a ContextRepository over the given store. The
// createInstance factory must return an aggregate whose inner-apply handler is
// installed (e.g. via domain.NewWithID) so rehydration can route events
// through it.
func NewContextRepository[T domain.IGenericIDAggregate[TID], TID any](
	store Store[TID],
	createInstance func() T,
) ContextRepository[T, TID] {
	return contextRepository[T, TID]{store: store, createInstance: createInstance}
}

func (r contextRepository[T, TID]) GetById(ctx context.Context, id TID) (T, error) {
	events, err := r.store.GetEventsForAggregate(ctx, id)
	if err != nil {
		var zero T
		return zero, err
	}
	if len(events) == 0 {
		var zero T
		return zero, ErrAggregateNotFound
	}
	agg := r.createInstance()
	applier := reflect.ValueOf(agg).Interface().(domain.InnerApplier)
	for _, e := range events {
		applier.InnerApply(e)
	}
	return agg, nil
}

func (r contextRepository[T, TID]) Save(ctx context.Context, aggregate T, expectedVersion int) error {
	if err := r.store.SaveEvents(
		ctx,
		reflect.TypeOf(aggregate).Name(),
		aggregate.Id(),
		aggregate.UncommittedEvents(),
		expectedVersion,
	); err != nil {
		return err
	}
	if committer, ok := any(aggregate).(interface{ MarkChangesAsCommitted() }); ok {
		committer.MarkChangesAsCommitted()
	}
	return nil
}
