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

func (i InventoryCommandHandlers) Handle(item CreateInventoryItem) error {
	inventoryItem := NewInventoryItem(item.InventoryItemId, item.Name)
	err := i.repository.Save(inventoryItem, -1)
	if err != nil {
		return err
	}
	return nil
}

func (i InventoryCommandHandlers) HandleCreateInventoryItem(cmd cqrs.Command) error {
	item := cmd.(CreateInventoryItem)
	inventoryItem := NewInventoryItem(item.InventoryItemId, item.Name)
	err := i.repository.Save(inventoryItem, -1)
	if err != nil {
		return err
	}
	return nil
}

func (i InventoryCommandHandlers) HandleRenameInventoryItem(cmd cqrs.Command) error {
	item := cmd.(RenameInventoryItem)
	inventoryItem, err := i.repository.GetById(item.InventoryItemId)
	if err != nil {
		return err
	}
	expectedVersion := inventoryItem.Version()
	inventoryItem.Rename(item.NewName)
	err = i.repository.Save(inventoryItem, expectedVersion)
	if err != nil {
		return err
	}
	return nil
}
