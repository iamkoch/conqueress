# Conqueress

A small CQRS and event-sourcing kit for Go in the ports-and-adapters style.
Inspired by Greg Young's "simplest possible thing" talks.

## What's here

### Core (repository root)

Domain primitives, dispatcher, eventing.

- `Event` interface and `BaseEvent`. Events carry an ID, version, correlation,
  causation and `OccurredAt`. Consumers embed `*BaseEvent` to satisfy the
  interface.
- `IntegrationEvent` interface. Extends `Event` with `EventType() string` for
  cross-language wire stability when an event has to leave the service.
- `Command` (marker) and `BaseCommand`. Commands optionally carry an ID,
  correlation, causation and `CreatedAt`. Embed `BaseCommand` when you want
  those fields populated.
- `HandleCommand[T]`, `HandleQuery[Q,R]`, `HandleEvent[T]`. Typed function
  aliases for handler composition. Factory functions close over dependencies
  and return one of these.
- `IntegrationEventPublisher` interface. Outbound port for publishing
  integration events to a message bus.
- `Mediator`. Reflection-based command and event dispatcher. Optional.

### `domain/`

`AggregateRootBase[TID]`. Embed in your aggregate. The base tracks uncommitted
changes and an injected `innerApply` callback.

Dispatch is a normal Go type switch that the consumer owns. Each aggregate
declares a `handleEvent(e cqrs.Event)` method containing a
`switch evt := e.(type)` over the events it handles, then calls
`SetInnerApply(handleEvent)` at construction. The base routes every event
through that callback. No reflection in the dispatch path.

### `eventstore/`

- `Repository[T]` and `GenericIDRepository[T, TID]`. `GetById(id)` and
  `Save(aggregate, expectedVersion)` with optimistic concurrency.
- `IEventStore` and `IGenericIDEventStore[TID]`. The persistence port.
- `inmemory/`. In-process implementation for tests.

### `guid/`

`guid.Guid`. xid-backed ID type with `New()`, `FromString` and
`MustFromString`.

### Sibling adapters

- `conqueress-mongo/`. MongoDB-backed event store with transactional saves.
- `conqueress-firestore/`. Firestore-backed event store.

## Quick start

### Define an aggregate

```go
package inventory

import (
    cqrs "github.com/iamkoch/conqueress"
    "github.com/iamkoch/conqueress/domain"
    "github.com/iamkoch/conqueress/guid"
)

type Item struct {
    domain.AggregateRootBase[guid.Guid]
    name string
}

// Two one-line helpers per aggregate so domain.New[Item]() can construct it.
func (i *Item) SetBase(b domain.AggregateRootBase[guid.Guid]) { i.AggregateRootBase = b }
func (i *Item) GetHandler() func(cqrs.Event)                  { return i.handleEvent }

type ItemCreated struct {
    *cqrs.BaseEvent
    Id   guid.Guid
    Name string
}

type ItemRenamed struct {
    *cqrs.BaseEvent
    NewName string
}

func NewItem(id guid.Guid, name string) *Item {
    item := domain.New[Item]()
    item.ApplyChange(cqrs.NewEvent[ItemCreated](func(e *ItemCreated) {
        e.Id = id
        e.Name = name
    }))
    return item
}

func (i *Item) handleEvent(e cqrs.Event) {
    switch evt := e.(type) {
    case ItemCreated:
        i.SetId(evt.Id)
        i.SetVersion(evt.Ver)
        i.name = evt.Name
    case ItemRenamed:
        i.name = evt.NewName
        i.SetVersion(evt.Ver)
    }
}

func (i *Item) Rename(name string) {
    i.ApplyChange(cqrs.NewEvent[ItemRenamed](func(e *ItemRenamed) {
        e.NewName = name
    }))
}
```

For aggregates with a non-`guid.Guid` ID, use `domain.NewWithID[Item, OrderID]()`
and have `SetBase` accept the right base type.

### Compose a command handler

```go
func MakeCreateItemHandler(repo Repository) cqrs.HandleCommand[CreateItem] {
    return func(ctx context.Context, cmd CreateItem, correlation, causation guid.Guid) error {
        item := NewItem(guid.New(), cmd.Name)
        return repo.Save(ctx, item, -1, correlation, causation)
    }
}
```

The HTTP command-handler module receives the typed
`HandleCommand[CreateItem]` at construction time and invokes it per request.

### Maintain a read-model projection

`BaseProjection` carries identity and a monotonic version. `BaseProjectionHandler[TProjection]`
wraps a `LoadProjection` and `SaveProjection` pair so the handler can express the update
as "load, mutate, save" without spelling it out at every call site:

```go
type ItemSummary struct {
    cqrs.BaseProjection
    Name string
}

type ItemSummaryHandler struct {
    cqrs.BaseProjectionHandler[*ItemSummary]
}

func NewItemSummaryHandler(load cqrs.LoadProjection[*ItemSummary], save cqrs.SaveProjection[*ItemSummary]) *ItemSummaryHandler {
    return &ItemSummaryHandler{
        BaseProjectionHandler: *cqrs.NewBaseProjectionHandler(load, save, func(id guid.Guid) *ItemSummary {
            p := &ItemSummary{}
            p.BaseProjection = cqrs.NewBaseProjection(id, -1)
            return p
        }),
    }
}

func (h *ItemSummaryHandler) OnCreated(e cqrs.Event) error {
    iic := e.(ItemCreated)
    return h.UpdateProjection(iic.Id, iic, func(p *ItemSummary, e cqrs.Event) {
        evt := e.(ItemCreated)
        p.Name = evt.Name
        p.IncrementVersion()
    })
}
```

The `LoadProjection` and `SaveProjection` delegates are the persistence port; pick any
store (Mongo, Postgres, in-memory) and supply closures that match the delegate signatures.

### Translate a domain event to an integration event

The integration-events ACL in your service subscribes to internal
`ItemCreated` events and publishes the wire-stable equivalent:

```go
type ItemCreatedIntegrationEvent struct {
    *cqrs.BaseEvent
    ItemId string
    Name   string
}

func (ItemCreatedIntegrationEvent) EventType() string {
    return "com.example.inventory.item-created"
}

func (a *Acl) OnItemCreated(ctx context.Context, evt ItemCreated, correlationId guid.Guid) error {
    integration := cqrs.NewEvent[ItemCreatedIntegrationEvent](func(e *ItemCreatedIntegrationEvent) {
        e.ItemId = evt.Id.String()
        e.Name = evt.Name
    })
    return a.publisher.Publish(ctx, integration, correlationId, evt.MsgId())
}
```

## Architecture

There is no central God-object. The Mediator is one option for in-process
dispatch. You can ignore it and compose handlers via factory functions that
return `HandleCommand[T]`.

Domain events stay inside the aggregate's bounded context. Integration events
cross service boundaries via a published bus. The translation between the two
lives in your ACL layer and is explicit.

Every event carries correlation, causation and `OccurredAt`. No envelope
wrapper is required; the metadata sits on the event itself.

Aggregate IDs are generic. `AggregateRootBase[TID]` and
`GenericIDRepository[T, TID]` accept any ID type. `guid.Guid` is the common
case. Composite or value-typed IDs such as a `(policyId, version)` tuple work
the same way.

## Tests

```sh
go test -race -count=1 ./...
```

CI runs the same command across the two actively supported Go releases
(currently 1.25 and 1.26) on every push and pull request. See
`.github/workflows/test.yml`.

## Patterns

Guidance notes on when to reach for which mechanism live in `doc/patterns/`.

- [Facts versus commands](doc/patterns/facts-vs-commands.md). When an
  inbound write is an observation rather than a decision, route it to
  idempotent durable storage rather than through commands and aggregates.

## Status

Experimental. The API is still evolving. Pin to a commit SHA in your `go.mod`
and update deliberately.

## Licence

See `LICENSE`.
