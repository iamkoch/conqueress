package store

import (
	"context"
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/example"
	"github.com/iamkoch/conqueress/guid"
	"github.com/iamkoch/ensure"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

// testPublisher is called on the mediator's own goroutines, so it guards its
// records with a mutex and hands out copies.
type testPublisher struct {
	mu             sync.Mutex
	capturedEvents []cqrs.Event
}

func newTestPublisher() *testPublisher {
	return &testPublisher{capturedEvents: make([]cqrs.Event, 0)}
}

func (t *testPublisher) Publish(event cqrs.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.capturedEvents = append(t.capturedEvents, event)
}

func (t *testPublisher) Handle(event cqrs.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.capturedEvents = append(t.capturedEvents, event)
	return nil
}

func (t *testPublisher) Captured() []cqrs.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]cqrs.Event(nil), t.capturedEvents...)
}

// The emulator accepts any project, so tests do not need a real one. Point
// FIRESTORE_PROJECT_ID somewhere else to run these against a real project.
func testProjectID() string {
	if p := os.Getenv("FIRESTORE_PROJECT_ID"); p != "" {
		return p
	}
	return "conqueress-test"
}

func TestConcurrencyBehaviour(t *testing.T) {
	var (
		aggregateId = guid.New()
		es          eventstore.IEventStore
		err         error
	)
	ensure.That("saving the same entity twice with the same expected version causes concurrency failures", func(s *ensure.Scenario) {
		s.Background("Given an available firestore event store", func() {
			tm := NewTypeMap().Add(example.InventoryItemCreated{}).Add(example.InventoryItemRenamed{})

			es, err = NewFirestoreEventStore(context.Background(), testProjectID(), tm)
			require.Nil(t, err, "should have failed to create event store")

		})

		s.Given("events saved", func() {

			err = es.SaveEvents(
				reflect.TypeOf(example.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					cqrs.NewEvent[example.InventoryItemCreated](func(s *example.InventoryItemCreated) {
						s.Id = guid.New()
						s.Name = "test"
					}),
				},
				-1,
			)

			require.Nil(t, err, "save should not have failed")
		})

		s.When("try to save again at same version", func() {

			err = es.SaveEvents(
				reflect.TypeOf(example.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					cqrs.NewEvent[example.InventoryItemCreated](func(s *example.InventoryItemCreated) {
						s.Id = guid.New()
						s.Name = "test"
					}),
				},
				-1,
			)
		})

		s.Then("should fail with concurrency error", func() {
			require.NotNil(t, err, "save should not have failed")
			require.ErrorIs(t, err, eventstore.ErrConcurrencyException)
		})

		s.When("try to save again at version one higher", func() {

			err = es.SaveEvents(
				reflect.TypeOf(example.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					cqrs.NewEvent[example.InventoryItemCreated](func(s *example.InventoryItemCreated) {
						s.Id = guid.New()
						s.Name = "test"
					}),
				},
				1,
			)
		})

		s.Then("should fail with concurrency error", func() {
			require.NotNil(t, err, "save should not have failed")
			require.ErrorIs(t, err, eventstore.ErrConcurrencyException)
		})
	}, t)
}

func TestVersionsAndConcurrency(t *testing.T) {
	var (
		aggregateId = guid.New()
		es          eventstore.IEventStore
		err         error
		aggEvents   []cqrs.Event
		lastVersion int
	)
	ensure.That("saving the same entity with variable event lengths causes correct version mismatch comparison", func(s *ensure.Scenario) {
		s.Background("Given an available firestore event store", func() {
			tm := NewTypeMap().Add(example.InventoryItemCreated{}).Add(example.InventoryItemRenamed{})

			es, err = NewFirestoreEventStore(context.Background(), testProjectID(), tm)
			require.Nil(t, err, "should have failed to create event store")

		})

		s.Given("events saved", func() {

			err = es.SaveEvents(
				reflect.TypeOf(example.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					cqrs.NewEvent[example.InventoryItemCreated](func(s *example.InventoryItemCreated) {
						s.Id = guid.New()
						s.Name = "test"
					}),
					cqrs.NewEvent[example.InventoryItemRenamed](func(s *example.InventoryItemRenamed) {
						s.Id = guid.New()
						s.NewName = "test2"
					}),
					cqrs.NewEvent[example.InventoryItemRenamed](func(s *example.InventoryItemRenamed) {
						s.Id = guid.New()
						s.NewName = "test3"
					}),
				},
				-1,
			)

			require.Nil(t, err, "save should not have failed")
		})

		s.When("loading", func() {
			aggEvents = es.GetEventsForAggregate(aggregateId)
		})

		s.Then("should have 3 events", func() {
			require.Equal(t, 3, len(aggEvents))
		})

		s.And("events should be types", func() {
			require.IsType(t, example.InventoryItemCreated{}, aggEvents[0])
			require.IsType(t, example.InventoryItemRenamed{}, aggEvents[1])
			require.IsType(t, example.InventoryItemRenamed{}, aggEvents[2])
		})

		s.And("events should contain correct info", func() {
			require.Equal(t, "test", aggEvents[0].(example.InventoryItemCreated).Name)
			require.Equal(t, "test2", aggEvents[1].(example.InventoryItemRenamed).NewName)
			require.Equal(t, "test3", aggEvents[2].(example.InventoryItemRenamed).NewName)
		})

		s.And("should have correct versions", func() {
			for i, e := range aggEvents {
				lastVersion = e.Version()
				require.Equal(t, i, e.Version(), fmt.Sprintf("version mismatch at index %d", i))
			}
		})

		s.And("when you try to save again at wrong version", func() {
			err = es.SaveEvents(
				reflect.TypeOf(example.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					cqrs.NewEvent[example.InventoryItemRenamed](func(s *example.InventoryItemRenamed) {
						s.Id = guid.New()
						s.NewName = "test22"
					}),
				},
				lastVersion+1,
			)
		})

		s.Then("should fail with concurrency error", func() {
			require.NotNil(t, err, "save should not have failed")
			require.ErrorIs(t, err, eventstore.ErrConcurrencyException)
		})
	}, t)
}

func TestConcurrency(t *testing.T) {
	Convey("saving the same entity twice with the same expected version", t, func() {
		m := cqrs.NewMediator(false)
		tm := NewTypeMap().Add(example.InventoryItemCreated{}).Add(example.InventoryItemRenamed{})

		s, err := NewFirestoreEventStore(context.Background(), testProjectID(), tm)
		if err != nil {
			panic(err.Error())
		}
		repo := eventstore.NewRepository[*example.InventoryItem](s, example.DefaultInventoryItem)
		commands := example.NewInventoryCommandHandler(repo)
		m.RegisterCommandHandler(reflect.TypeOf(example.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
		m.RegisterCommandHandler(reflect.TypeOf(example.RenameInventoryItem{}), commands.HandleRenameInventoryItem)

		handler := newTestPublisher()
		m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemCreated{}), handler.Handle)
		m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemRenamed{}), handler.Handle)

		Convey("should throw a concurrency error", func() {
			itemId := guid.New()
			item := example.NewInventoryItem(itemId, "test")
			e := repo.Save(item, -1)
			So(e, ShouldBeNil)

			a, getErr := repo.GetById(item.AggregateRootBase.Id())
			So(getErr, ShouldBeNil)
			a.Rename("test2")
			e = repo.Save(a, 3)
			So(e, ShouldNotBeNil)

			// The rejected write must leave the stored stream untouched.
			reloaded, reloadErr := repo.GetById(itemId)
			So(reloadErr, ShouldBeNil)
			So(reloaded.Name(), ShouldEqual, "test")
		})
	})
}

func TestStore(t *testing.T) {
	Convey("save and load should work simply", t, func() {
		Convey("when saving", func() {
			m := cqrs.NewMediator(false)
			tm := NewTypeMap().Add(example.InventoryItemCreated{}).Add(example.InventoryItemRenamed{})

			s, err := NewFirestoreEventStore(context.Background(), testProjectID(), tm)
			if err != nil {
				panic(err.Error())
			}
			repo := eventstore.NewRepository[*example.InventoryItem](s, example.DefaultInventoryItem)
			commands := example.NewInventoryCommandHandler(repo)
			m.RegisterCommandHandler(reflect.TypeOf(example.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
			m.RegisterCommandHandler(reflect.TypeOf(example.RenameInventoryItem{}), commands.HandleRenameInventoryItem)

			handler := newTestPublisher()
			m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemCreated{}), handler.Handle)
			time.Sleep(time.Second)
			actualId := guid.New()
			err = m.Dispatch(example.NewCreateInventoryItem(actualId, "something"), nil)
			So(err, ShouldBeNil)
			time.Sleep(time.Second * 2)

			ii, getErr := repo.GetById(actualId)
			So(getErr, ShouldBeNil)
			So(ii.Name(), ShouldEqual, "something")

			err = m.Dispatch(example.NewRenameInventoryItem(actualId, "something new"), nil)
			So(err, ShouldBeNil)
			time.Sleep(time.Second * 2)

			ii, getErr = repo.GetById(actualId)
			So(getErr, ShouldBeNil)
			So(ii.Name(), ShouldEqual, "something new")
		})
	})

	Convey("concurrency check works", t, func() {
		Convey("when saving with wrong version, throws concurrency exception", func() {
			m := cqrs.NewMediator(false)
			tm := NewTypeMap().Add(example.InventoryItemCreated{}).Add(example.InventoryItemRenamed{})

			s, err := NewFirestoreEventStore(context.Background(), testProjectID(), tm)
			if err != nil {
				panic(err.Error())
			}
			repo := eventstore.NewRepository[*example.InventoryItem](s, example.DefaultInventoryItem)
			commands := example.NewInventoryCommandHandler(repo)
			m.RegisterCommandHandler(reflect.TypeOf(example.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
			m.RegisterCommandHandler(reflect.TypeOf(example.RenameInventoryItem{}), commands.HandleRenameInventoryItem)

			handler := newTestPublisher()
			m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemCreated{}), handler.Handle)
			time.Sleep(time.Second)
			actualId := guid.New()
			err = m.Dispatch(example.NewCreateInventoryItem(actualId, "something"), nil)
			So(err, ShouldBeNil)
			time.Sleep(time.Second * 2)

			ii, getErr := repo.GetById(actualId)
			So(getErr, ShouldBeNil)
			So(ii.Name(), ShouldEqual, "something")

			err = m.Dispatch(example.NewRenameInventoryItem(actualId, "something new"), nil)
			err = m.Dispatch(example.NewRenameInventoryItem(actualId, "something new 2"), nil)
			err = m.Dispatch(example.NewRenameInventoryItem(actualId, "something new 3"), nil)
			So(err, ShouldBeNil)
			time.Sleep(time.Second * 2)

			ii, getErr = repo.GetById(actualId)
			So(getErr, ShouldBeNil)
			So(ii.Name(), ShouldEqual, "something new 3")
		})
	})
}

// TestConcurrentWritersAtSameVersion checks that the version check is part of
// the transaction. Several writers load the same aggregate, so they all hold
// the same expected version, and all try to write at once. Exactly one may
// win. If the read that feeds checkConcurrency happens outside the
// transaction, more than one writer sees a stale version and commits.
func TestConcurrentWritersAtSameVersion(t *testing.T) {
	const writers = 8

	tm := NewTypeMap().Add(example.InventoryItemCreated{}).Add(example.InventoryItemRenamed{})

	s, err := NewFirestoreEventStore(context.Background(), testProjectID(), tm)
	require.NoError(t, err)

	repo := eventstore.NewRepository[*example.InventoryItem](s, example.DefaultInventoryItem)

	itemId := guid.New()
	require.NoError(t, repo.Save(example.NewInventoryItem(itemId, "original"), -1))

	loaded := make([]*example.InventoryItem, writers)
	for i := range loaded {
		item, err := repo.GetById(itemId)
		require.NoError(t, err)
		loaded[i] = item
	}

	expectedVersion := loaded[0].Version()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]error, writers)
		start   = make(chan struct{})
	)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			item := loaded[i]
			item.Rename(fmt.Sprintf("renamed by %d", i))

			<-start
			e := repo.Save(item, expectedVersion)

			mu.Lock()
			defer mu.Unlock()
			results[i] = e
		}(i)
	}

	close(start)
	wg.Wait()

	succeeded := 0
	for _, e := range results {
		if e == nil {
			succeeded++
		}
	}

	require.Equal(t, 1, succeeded,
		"exactly one writer may commit at version %d, but %d did", expectedVersion, succeeded)

	reloaded, err := repo.GetById(itemId)
	require.NoError(t, err)
	require.Equal(t, expectedVersion+1, reloaded.Version(),
		"the stream must have advanced by exactly one event")
}
