package e2e

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/eventstore/inmemory"
	"github.com/iamkoch/conqueress/example"
	"github.com/iamkoch/conqueress/guid"
	. "github.com/smartystreets/goconvey/convey"
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

func TestApplication(t *testing.T) {

	Convey("Create inventory item", t, func() {
		m := cqrs.NewMediator(false)
		storage := inmemory.NewInMemoryEventStore[guid.Guid](m)
		repo := eventstore.NewRepository[*example.InventoryItem](storage, example.DefaultInventoryItem)

		commands := example.NewInventoryCommandHandler(repo)
		handler := newTestPublisher()
		cqrs.RegisterCommandHandler[example.CreateInventoryItem](m, commands.HandleCreateInventoryItem)
		cqrs.RegisterEventHandlers[example.InventoryItemCreated](m, handler.Handle)

		actualId := guid.New()
		m.Dispatch(example.NewCreateInventoryItem(actualId, "something"), nil)
		time.Sleep(time.Second)
		So(len(handler.Captured()), ShouldEqual, 1)
		firstEvent := handler.Captured()[0]
		iic := firstEvent.(example.InventoryItemCreated)
		So(iic.Id, ShouldEqual, actualId)
		So(iic.Name, ShouldEqual, "something")
	})

	Convey("Applying multiple commands", t, func() {
		m := cqrs.NewMediator(false)
		storage := inmemory.NewInMemoryEventStore[guid.Guid](m)
		repo := eventstore.NewRepository[*example.InventoryItem](storage, example.DefaultInventoryItem)

		commands := example.NewInventoryCommandHandler(repo)
		handler := newTestPublisher()
		m.RegisterCommandHandler(reflect.TypeOf(example.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
		m.RegisterCommandHandler(reflect.TypeOf(example.RenameInventoryItem{}), commands.HandleRenameInventoryItem)
		m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemCreated{}), handler.Handle)
		m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemRenamed{}), handler.Handle)

		inventoryItemId := guid.New()
		m.Dispatch(example.NewCreateInventoryItem(inventoryItemId, "something"), nil)
		m.Dispatch(example.NewRenameInventoryItem(inventoryItemId, "something new"), nil)
		time.Sleep(time.Second)

		So(len(handler.Captured()), ShouldEqual, 2)
	})

	Convey("Mediator blows when same handler registered twice", t, func() {
		m := cqrs.NewMediator(false)

		handler := newTestPublisher()
		m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemRenamed{}), handler.Handle)
		m.RegisterEventHandler(reflect.TypeOf(example.InventoryItemRenamed{}), handler.Handle)

	})

}
