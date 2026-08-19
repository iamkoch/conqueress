package e2e

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/eventstore/inmemory"
	"github.com/iamkoch/conqueress/guid"
	. "github.com/smartystreets/goconvey/convey"
	"reflect"
	"sample_domain"
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
		repo := eventstore.NewRepository[*sample_domain.InventoryItem](storage, sample_domain.DefaultInventoryItem)

		commands := sample_domain.NewInventoryCommandHandler(repo)
		handler := newTestPublisher()
		cqrs.RegisterCommandHandler[sample_domain.CreateInventoryItem](m, commands.HandleCreateInventoryItem)
		cqrs.RegisterEventHandlers[sample_domain.InventoryItemCreated](m, handler.Handle)

		actualId := guid.New()
		m.Dispatch(sample_domain.NewCreateInventoryItem(actualId, "something"), nil)
		time.Sleep(time.Second)
		So(len(handler.Captured()), ShouldEqual, 1)
		firstEvent := handler.Captured()[0]
		iic := firstEvent.(sample_domain.InventoryItemCreated)
		So(iic.Id, ShouldEqual, actualId)
		So(iic.Name, ShouldEqual, "something")
	})

	Convey("Applying multiple commands", t, func() {
		m := cqrs.NewMediator(false)
		storage := inmemory.NewInMemoryEventStore[guid.Guid](m)
		repo := eventstore.NewRepository[*sample_domain.InventoryItem](storage, sample_domain.DefaultInventoryItem)

		commands := sample_domain.NewInventoryCommandHandler(repo)
		handler := newTestPublisher()
		m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
		m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)
		m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
		m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemRenamed{}), handler.Handle)

		inventoryItemId := guid.New()
		m.Dispatch(sample_domain.NewCreateInventoryItem(inventoryItemId, "something"), nil)
		m.Dispatch(sample_domain.NewRenameInventoryItem(inventoryItemId, "something new"), nil)
		time.Sleep(time.Second)

		So(len(handler.Captured()), ShouldEqual, 2)
	})

	Convey("Mediator blows when same handler registered twice", t, func() {
		m := cqrs.NewMediator(false)

		handler := newTestPublisher()
		m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemRenamed{}), handler.Handle)
		m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemRenamed{}), handler.Handle)

	})

}
