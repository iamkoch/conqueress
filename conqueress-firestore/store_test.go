package store

import (
	"context"
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	. "github.com/smartystreets/goconvey/convey"
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
