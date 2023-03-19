package conqueress

import "github.com/iamkoch/conqueress/guid"

type Projection interface {
	Id() guid.Guid
	Version() int
	IncrementVersion()
}

type BaseProjection struct {
	id      guid.Guid
	version int
}

func NewBaseProjection(id guid.Guid, version int) BaseProjection {
	return BaseProjection{
		id,
		version,
	}
}

func (p *BaseProjection) Id() guid.Guid {
	return p.id
}

func (p *BaseProjection) Version() int {
	return p.version
}

func (p *BaseProjection) IncrementVersion() {
	p.version += 1
}

type LoadProjection[TProjection Projection] func(id guid.Guid) (TProjection, error)

type SaveProjection[TProjection Projection] func(projection TProjection) error

type BaseProjectionHandler[TProjection Projection] struct {
	load              LoadProjection[TProjection]
	save              SaveProjection[TProjection]
	projectionFactory func(id guid.Guid) TProjection
}

func NewBaseProjectionHandler[TProjection Projection](
	load LoadProjection[TProjection],
	save SaveProjection[TProjection],
	factory func(id guid.Guid) TProjection) *BaseProjectionHandler[TProjection] {
	return &BaseProjectionHandler[TProjection]{load, save, factory}
}

func (bh BaseProjectionHandler[TProjection]) UpdateProjection(
	id guid.Guid,
	evt Event,
	toDo func(p TProjection, e Event),
) error {
	p, e := bh.load(id)

	if e != nil {
		return e
	}

	toDo(p, evt)

	e = bh.save(p)

	if e != nil {
		return e
	}

	return nil
}
