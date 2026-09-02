// Package example implements a small inventory aggregate, with the commands,
// events, command handlers, and read model that go with it.
//
// It exists for the library's own tests and for the worked example in the
// README. It is not part of the stable API: anything here can change in any
// release, so copy what you need rather than importing it.
package example

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/domain"
	"github.com/iamkoch/conqueress/guid"
)

type InventoryItem struct {
	domain.AggregateRootBase[guid.Guid]
	name string
	id   guid.Guid
}

func NewInventoryItem(id guid.Guid, name string) *InventoryItem {
	i := DefaultInventoryItem()

	i.ApplyChange(cqrs.NewEvent[InventoryItemCreated](func(e *InventoryItemCreated) {
		e.Id = id
		e.Name = name
	}))

	return i
}

func (ii *InventoryItem) Name() string {
	return ii.name
}

func DefaultInventoryItem() *InventoryItem {
	ii := InventoryItem{
		AggregateRootBase: domain.NewAggregate[guid.Guid](),
	}
	ii.SetInnerApply(ii.handleEvent)
	return &ii
}

type InventoryItemCreated struct {
	*cqrs.BaseEvent
	Id   guid.Guid
	Name string
}

type InventoryItemRenamed struct {
	*cqrs.BaseEvent
	Id      guid.Guid
	NewName string
}

func (ii *InventoryItem) handleEvent(e cqrs.Event) {
	switch evt := e.(type) {
	case InventoryItemCreated:
		ii.SetId(evt.Id)
		ii.name = evt.Name
	case InventoryItemRenamed:
		ii.name = evt.NewName
	}
}

func (ii *InventoryItem) Rename(name string) {
	ii.ApplyChange(cqrs.NewEvent[InventoryItemRenamed](func(e *InventoryItemRenamed) {
		e.Id = ii.Id()
		e.NewName = name
	}))
}
