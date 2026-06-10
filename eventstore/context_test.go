package eventstore_test

import (
	"context"
	"errors"
	"testing"

	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/domain"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/stretchr/testify/assert"
)

// fakeStore is a minimal in-memory Store[string] for exercising the
// repository contract, including version assignment and concurrency failure.
type fakeStore struct {
	streams map[string][]cqrs.Event
	failGet error
}

func newFakeStore() *fakeStore {
	return &fakeStore{streams: map[string][]cqrs.Event{}}
}

func (s *fakeStore) SaveEvents(_ context.Context, _ string, id string, events []cqrs.Event, expectedVersion int) error {
	current := len(s.streams[id]) - 1
	if expectedVersion != -1 && current != expectedVersion {
		return eventstore.ErrConcurrencyException
	}
	v := expectedVersion
	for _, e := range events {
		v++
		e.WithVersion(v)
		s.streams[id] = append(s.streams[id], e)
	}
	return nil
}

func (s *fakeStore) GetEventsForAggregate(_ context.Context, id string) ([]cqrs.Event, error) {
	if s.failGet != nil {
		return nil, s.failGet
	}
	return s.streams[id], nil
}

type counterCreated struct {
	*cqrs.BaseEvent
	Name string
}

type counterIncremented struct {
	*cqrs.BaseEvent
}

type counter struct {
	domain.AggregateRootBase[string]
	name  string
	count int
}

func (c *counter) SetBase(b domain.AggregateRootBase[string]) { c.AggregateRootBase = b }
func (c *counter) GetHandler() func(cqrs.Event)               { return c.handleEvent }

func (c *counter) handleEvent(e cqrs.Event) {
	switch evt := e.(type) {
	case counterCreated:
		c.SetId("counter-1")
		c.SetVersion(evt.Ver)
		c.name = evt.Name
	case counterIncremented:
		c.count++
		c.SetVersion(evt.Ver)
	}
}

func newCounter() *counter { return domain.NewWithID[counter, string]() }

func TestContextRepository_SaveAssignsVersionsAndCommits(t *testing.T) {
	store := newFakeStore()
	repo := eventstore.NewContextRepository[*counter, string](store, newCounter)

	c := newCounter()
	c.ApplyChange(cqrs.NewEvent[counterCreated](func(e *counterCreated) { e.Name = "hits" }))
	c.ApplyChange(cqrs.NewEvent[counterIncremented]())

	assert.NoError(t, repo.Save(context.Background(), c, -1))
	assert.Empty(t, c.UncommittedEvents(), "save marks changes committed")

	stream := store.streams["counter-1"]
	assert.Len(t, stream, 2)
	assert.Equal(t, 0, stream[0].Version())
	assert.Equal(t, 1, stream[1].Version())
}

func TestContextRepository_GetByIdRehydrates(t *testing.T) {
	store := newFakeStore()
	repo := eventstore.NewContextRepository[*counter, string](store, newCounter)

	c := newCounter()
	c.ApplyChange(cqrs.NewEvent[counterCreated](func(e *counterCreated) { e.Name = "hits" }))
	c.ApplyChange(cqrs.NewEvent[counterIncremented]())
	c.ApplyChange(cqrs.NewEvent[counterIncremented]())
	assert.NoError(t, repo.Save(context.Background(), c, -1))

	loaded, err := repo.GetById(context.Background(), "counter-1")
	assert.NoError(t, err)
	assert.Equal(t, "hits", loaded.name)
	assert.Equal(t, 2, loaded.count)
	assert.Equal(t, 2, loaded.Version(), "aggregate version reflects last event")
}

func TestContextRepository_GetByIdEmptyStreamIsNotFound(t *testing.T) {
	repo := eventstore.NewContextRepository[*counter, string](newFakeStore(), newCounter)

	_, err := repo.GetById(context.Background(), "missing")
	assert.ErrorIs(t, err, eventstore.ErrAggregateNotFound)
}

func TestContextRepository_GetByIdPropagatesStoreError(t *testing.T) {
	store := newFakeStore()
	store.failGet = errors.New("connection reset")
	repo := eventstore.NewContextRepository[*counter, string](store, newCounter)

	_, err := repo.GetById(context.Background(), "counter-1")
	assert.ErrorContains(t, err, "connection reset")
}

func TestContextRepository_SaveConcurrencyConflict(t *testing.T) {
	store := newFakeStore()
	repo := eventstore.NewContextRepository[*counter, string](store, newCounter)

	c := newCounter()
	c.ApplyChange(cqrs.NewEvent[counterCreated](func(e *counterCreated) { e.Name = "hits" }))
	assert.NoError(t, repo.Save(context.Background(), c, -1))

	// A second writer saves at a stale expectedVersion.
	stale := newCounter()
	stale.ApplyChange(cqrs.NewEvent[counterCreated](func(e *counterCreated) { e.Name = "clash" }))
	err := repo.Save(context.Background(), stale, 5)
	assert.ErrorIs(t, err, eventstore.ErrConcurrencyException)
}
