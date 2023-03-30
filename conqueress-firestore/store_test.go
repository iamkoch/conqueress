package store

import (
	"context"
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	enshur "github.com/iamkoch/ensure/stateless"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"
	"reflect"
	"sample_domain"
	"testing"
	"time"
)

type testPublisher struct {
	capturedEvents []cqrs.Event
}

func newTestPublisher() *testPublisher {
	return &testPublisher{capturedEvents: make([]cqrs.Event, 0)}
}

func (t *testPublisher) Publish(event cqrs.Event) {
	t.capturedEvents = append(t.capturedEvents, event)
}

func (t *testPublisher) Handle(event cqrs.Event) error {
	t.capturedEvents = append(t.capturedEvents, event)
	return nil
}

func TestConcurrencyBehaviour(t *testing.T) {
	var (
		aggregateId = guid.New()
		es          eventstore.IEventStore
		err         error
	)
	enshur.That("saving the same entity twice with the same expected version causes concurrency failures", func(s *enshur.Scenario) {
		s.Background("Given an available firestore event store", func() {
			tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})

			es, err = NewFirestoreEventStore(context.Background(), tm)
			require.Nil(t, err, "should have failed to create event store")

		})

		s.Given("events saved", func() {

			err = es.SaveEvents(
				reflect.TypeOf(sample_domain.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					sample_domain.InventoryItemCreated{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), Name: "test"},
				},
				-1,
			)

			require.Nil(t, err, "save should not have failed")
		})

		s.When("try to save again at same version", func() {

			err = es.SaveEvents(
				reflect.TypeOf(sample_domain.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					sample_domain.InventoryItemCreated{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), Name: "test"},
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
				reflect.TypeOf(sample_domain.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					sample_domain.InventoryItemCreated{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), Name: "test"},
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
	enshur.That("saving the same entity with variable event lengths causes correct version mismatch comparison", func(s *enshur.Scenario) {
		s.Background("Given an available firestore event store", func() {
			tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})

			es, err = NewFirestoreEventStore(context.Background(), tm)
			require.Nil(t, err, "should have failed to create event store")

		})

		s.Given("events saved", func() {

			err = es.SaveEvents(
				reflect.TypeOf(sample_domain.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					sample_domain.InventoryItemCreated{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), Name: "test"},
					sample_domain.InventoryItemRenamed{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), NewName: "test2"},
					sample_domain.InventoryItemRenamed{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), NewName: "test3"},
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
			require.IsType(t, sample_domain.InventoryItemCreated{}, aggEvents[0])
			require.IsType(t, sample_domain.InventoryItemRenamed{}, aggEvents[1])
			require.IsType(t, sample_domain.InventoryItemRenamed{}, aggEvents[2])
		})

		s.And("events should contain correct info", func() {
			require.Equal(t, "test", aggEvents[0].(sample_domain.InventoryItemCreated).Name)
			require.Equal(t, "test2", aggEvents[1].(sample_domain.InventoryItemRenamed).NewName)
			require.Equal(t, "test3", aggEvents[2].(sample_domain.InventoryItemRenamed).NewName)
		})

		s.And("should have correct versions", func() {
			for i, e := range aggEvents {
				lastVersion = e.Version()
				require.Equal(t, i, e.Version(), fmt.Sprintf("version mismatch at index %d", i))
			}
		})

		s.And("when you try to save again at wrong version", func() {
			err = es.SaveEvents(
				reflect.TypeOf(sample_domain.InventoryItem{}).Name(),
				aggregateId,
				[]cqrs.Event{
					sample_domain.InventoryItemRenamed{BaseEvent: cqrs.DefaultBaseEvent(), Id: guid.New(), NewName: "test22"},
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
		tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})

		s, err := NewFirestoreEventStore(context.Background(), tm)
		if err != nil {
			panic(err.Error())
		}
		repo := eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
		commands := sample_domain.NewInventoryCommandHandler(repo)
		m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
		m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)

		handler := newTestPublisher()
		m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
		m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemRenamed{}), handler.Handle)

		Convey("should throw a concurrency error", func() {
			itemId := guid.New()
			item := sample_domain.NewInventoryItem(itemId, "test")
			e := repo.Save(item, -1)
			So(e, ShouldBeNil)

			a := repo.GetById(item.AggregateRootBase.Id())
			a.Rename("test2")
			e = repo.Save(a, 3)
			So(e, ShouldNotBeNil)
			So(len(handler.capturedEvents), ShouldEqual, 1)
		})
	})
}

func TestStore(t *testing.T) {
	Convey("save and load should work simply", t, func() {
		Convey("when saving", func() {
			m := cqrs.NewMediator(false)
			tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})

			s, err := NewFirestoreEventStore(context.Background(), tm)
			if err != nil {
				panic(err.Error())
			}
			repo := eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
			commands := sample_domain.NewInventoryCommandHandler(repo)
			m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
			m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)

			handler := newTestPublisher()
			m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
			time.Sleep(time.Second)
			actualId := guid.New()
			err = m.Dispatch(sample_domain.NewCreateInventoryItem(actualId, "something"), nil)
			if err != nil {
				fmt.Errorf("something went wrong dispatching %s", err.Error())
			}
			time.Sleep(time.Second * 2)

			ii := repo.GetById(actualId)
			So(ii.Name(), ShouldEqual, "something")

			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new"), nil)
			if err != nil {
				fmt.Errorf("something went wrong dispatching %s", err.Error())
			}
			time.Sleep(time.Second * 2)

			ii = repo.GetById(actualId)
			So(ii.Name(), ShouldEqual, "something new")
		})
	})

	Convey("concurrency check works", t, func() {
		Convey("when saving with wrong version, throws concurrency exception", func() {
			m := cqrs.NewMediator(false)
			tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})

			s, err := NewFirestoreEventStore(context.Background(), tm)
			if err != nil {
				panic(err.Error())
			}
			repo := eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
			commands := sample_domain.NewInventoryCommandHandler(repo)
			m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
			m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)

			handler := newTestPublisher()
			m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
			time.Sleep(time.Second)
			actualId := guid.New()
			err = m.Dispatch(sample_domain.NewCreateInventoryItem(actualId, "something"), nil)
			if err != nil {
				fmt.Errorf("something went wrong dispatching %s", err.Error())
			}
			time.Sleep(time.Second * 2)

			ii := repo.GetById(actualId)
			So(ii.Name(), ShouldEqual, "something")

			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new"), nil)
			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new 2"), nil)
			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new 3"), nil)
			if err != nil {
				fmt.Errorf("something went wrong dispatching %s", err.Error())
			}
			time.Sleep(time.Second * 2)

			ii = repo.GetById(actualId)
			So(ii.Name(), ShouldEqual, "something new 3")
		})
	})
}
