# Conqueress

A small ports-and-adapters CQRS / event-sourcing kit for Go. Inspired by Greg
Young's "simplest possible thing" talks and the C# pattern at
[`iamkoch/platform`](https://github.com/iamkoch/platform/tree/main/libraries/IntelAgent.Framework).

## What's in the box

### `conqueress/`

The core. Domain primitives, dispatcher, eventing.

- **`Event` interface + `BaseEvent`** — events carry their ID, version,
  correlation, causation and `OccurredAt`. Consumers embed `*BaseEvent` for
  free implementations.
- **`IntegrationEvent` interface** — extends `Event` with `EventType() string`
  for cross-language / cross-service wire stability. Distinct from domain
  events that stay inside the aggregate.
- **`Command` (marker) + `BaseCommand`** — commands optionally carry their
  own ID, correlation, causation and createdAt; embed `BaseCommand` for the
  free implementation.
- **`HandleCommand[T]`, `HandleQuery[Q,R]`, `HandleEvent[T]`** — typed
  function aliases for handler composition. Factory functions close over
  dependencies and return the right handler shape.
- **`IntegrationEventPublisher` interface** — the outbound port for
  publishing integration events to a message bus.
- **`Mediator`** — reflection-based command/event dispatcher. Use it for
  in-process routing when you want loose coupling; use the typed handlers
  above when you want statically-typed factory composition.

### `conqueress/domain/`

- **`AggregateRootBase[TID]`** — embed this in your aggregate. The base
  tracks uncommitted changes and an injected `innerApply` callback.
- **Dispatch pattern**: each aggregate writes its own
  `handleEvent(e cqrs.Event)` method containing a `switch evt := e.(type)`
  over the event types it handles, then calls `SetInnerApply(handleEvent)`
  at construction. No reflection in the dispatch path.

### `conqueress/eventstore/`

- **`Repository[T]` + `GenericIDRepository[T, TID]`** — `GetById(id)` /
  `Save(aggregate, expectedVersion)` with optimistic concurrency control.
- **`IEventStore` / `IGenericIDEventStore[TID]`** — the persistence port.
- **`inmemory/`** — in-process implementation for tests.

### `conqueress/guid/`

- **`guid.Guid`** — xid-backed ID type with `New()` / `FromString` /
  `MustFromString`.

### Sibling Mongo + Firestore implementations

- **`conqueress-mongo/`** — MongoDB-backed event store with transactional
  saves.
- **`conqueress-firestore/`** — Firestore-backed event store.

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
    item := &Item{AggregateRootBase: domain.NewAggregate[guid.Guid]()}
    item.SetInnerApply(item.handleEvent)
    item.ApplyChange(cqrs.NewEvent[ItemCreated](func(e *ItemCreated) {
        e.Id = id
        e.Name = name
    }))
    return item
}

// handleEvent is the type-switch the consumer owns. The base struct's
// ApplyChange routes every event through this method.
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

### Compose a command handler

```go
func MakeCreateItemHandler(repo Repository) cqrs.HandleCommand[CreateItem] {
    return func(ctx context.Context, cmd CreateItem, correlation, causation guid.Guid) error {
        item := NewItem(guid.New(), cmd.Name)
        return repo.Save(ctx, item, -1, correlation, causation)
    }
}
```

The HTTP-command-handler module receives the typed `HandleCommand[CreateItem]`
at construction time and invokes it per request.

### Map a domain event to an integration event

The integrationevents ACL in your service subscribes to internal `ItemCreated`
events and translates them into the public wire-stable event:

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

- **No central God-object.** The Mediator exists as a convenience for
  in-process dispatch; you don't have to use it. Typed function aliases
  (`HandleCommand[T]`, `HandleEvent[T]`) let you compose handlers via
  factories that close over their dependencies.
- **Domain Event vs Integration Event** as first-class. Domain events stay
  inside the aggregate's bounded context. Integration events cross service
  boundaries via a published bus. The translation between the two is
  explicit and lives in your ACL layer.
- **Correlation, causation, occurredAt on every event.** No envelope wrapper
  required; the metadata lives on the event itself.
- **Generic aggregate IDs.** `AggregateRootBase[TID]` and
  `GenericIDRepository[T, TID]` accept arbitrary ID types. `guid.Guid` is
  the common case; composite/value-typed IDs (e.g. `PolicyVersion`) are
  first-class.

## Status

Experimental. The framework is being actively shaped by use in extracting a
bind capability from a larger Spring service into a Go service. Expect the
API to evolve; pin to a commit SHA in your `go.mod` and update deliberately.
