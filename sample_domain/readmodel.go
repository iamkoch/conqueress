package sample_domain

import cqrs "github.com/iamkoch/conqueress"

type InventoryItemReadModel struct {
	cqrs.BaseProjection
	name string
}

type InventoryItemReadModelHandler struct {
	cqrs.BaseProjectionHandler[*InventoryItemReadModel]
}

func (i InventoryItemReadModelHandler) HandleCreated(e cqrs.Event) error {
	iic := e.(InventoryItemCreated)

	return i.UpdateProjection(iic.Id, iic, func(p *InventoryItemReadModel, e cqrs.Event) {
		p.BaseProjection = cqrs.NewBaseProjection(
			iic.Id,
			iic.Ver,
		)
		p.name = iic.Name
	})
}

func (i InventoryItemReadModelHandler) HandleRenamed(e cqrs.Event) error {
	iir := e.(InventoryItemRenamed)

	return i.UpdateProjection(iir.Id, iir, func(p *InventoryItemReadModel, e cqrs.Event) {
		p.name = iir.NewName
	})
}
