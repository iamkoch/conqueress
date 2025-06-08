package sample_domain

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/domain"
	"github.com/iamkoch/conqueress/guid"
)

type InventoryItem struct {
	domain.AggregateRootBase
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
		AggregateRootBase: domain.NewAggregate(),
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
		ii.SetVersion(evt.Ver)
		ii.name = evt.Name
	case InventoryItemRenamed:
		ii.name = evt.NewName
		ii.SetVersion(evt.Ver)
	}
}

func (ii *InventoryItem) Rename(name string) {
	ii.ApplyChange(cqrs.NewEvent[InventoryItemRenamed](func(e *InventoryItemRenamed) {
		e.Id = ii.Id()
		e.NewName = name
	}))
}
