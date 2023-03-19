package sample_domain

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
)

type InventoryCommandHandlers struct {
	repository eventstore.Repository[*InventoryItem]
}

func NewInventoryCommandHandler(repository eventstore.Repository[*InventoryItem]) InventoryCommandHandlers {
	return InventoryCommandHandlers{repository}
}

func (i InventoryCommandHandlers) Handle(item CreateInventoryItem) {
	inventoryItem := NewInventoryItem(item.InventoryItemId, item.Name)
	i.repository.Save(inventoryItem, -1)
}

func (i InventoryCommandHandlers) HandleCreateInventoryItem(cmd cqrs.Command) error {
	item := cmd.(CreateInventoryItem)
	inventoryItem := NewInventoryItem(item.InventoryItemId, item.Name)
	i.repository.Save(inventoryItem, -1)
	return nil
}

func (i InventoryCommandHandlers) HandleRenameInventoryItem(cmd cqrs.Command) error {
	item := cmd.(RenameInventoryItem)
	inventoryItem := i.repository.GetById(item.InventoryItemId)
	inventoryItem.Rename(item.NewName)
	i.repository.Save(inventoryItem, -1)
	return nil
}
