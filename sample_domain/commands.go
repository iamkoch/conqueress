package sample_domain

import (
	"github.com/iamkoch/conqueress/guid"
)

type CreateInventoryItem struct {
	InventoryItemId guid.Guid
	Name            string
}

type RenameInventoryItem struct {
	InventoryItemId guid.Guid
	NewName         string
}

func NewCreateInventoryItem(id guid.Guid, name string) CreateInventoryItem {
	return CreateInventoryItem{id, name}
}

func NewRenameInventoryItem(id guid.Guid, newName string) RenameInventoryItem {
	return RenameInventoryItem{
		InventoryItemId: id,
		NewName:         newName,
	}
}
