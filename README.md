# Conqueress

[![CI](https://github.com/iamkoch/conqueress/actions/workflows/ci.yml/badge.svg)](https://github.com/iamkoch/conqueress/actions/workflows/ci.yml)

Conqueress is a ports-and-adapters CQRS and event sourcing framework for Go. It
borrows heavily from the .NET space, so parts of it are not the most idiomatic
Go you will read. The persistence store is the part that benefits most: an
aggregate knows nothing about where its events end up, and you swap Firestore
for MongoDB or an in-memory store by passing a different event store to the
repository.

## Install

```sh
go get github.com/iamkoch/conqueress
```

The storage adapters are separate modules, so install only the one you need:

```sh
go get github.com/iamkoch/conqueress/firestore
go get github.com/iamkoch/conqueress/mongo
```

The core module requires Go 1.23 or later, because the aggregate and repository
types are generic. Both storage adapters require Go 1.25 or later, because the
patched versions of `golang.org/x/crypto` and `golang.org/x/net` do.

## Modules

| Module | Contents |
| --- | --- |
| `github.com/iamkoch/conqueress` | Mediator, events, aggregates, repositories, projections, the in-memory event store, and the sample domain |
| `github.com/iamkoch/conqueress/firestore` | Firestore event store |
| `github.com/iamkoch/conqueress/mongo` | MongoDB event store |

Both adapters declare `package store`, so alias the import if you use them
together.

The core module holds these packages:

- `conqueress` — the mediator, the `Event` and `Command` types, and projections.
- `conqueress/domain` — `AggregateRootBase` and the aggregate interfaces.
- `conqueress/eventstore` — the repository, and the event store interfaces the
  adapters implement.
- `conqueress/eventstore/inmemory` — an event store that keeps everything in a
  map, for tests.
- `conqueress/guid` — the identifier type, a thin wrapper over `xid`.
- `conqueress/sample_domain` — a worked inventory example, used by the adapter
  tests.

## Defining an aggregate

An aggregate embeds `domain.AggregateRootBase[TID]`, where `TID` is the type of
its identifier. Use `guid.Guid` unless you have a reason not to.

```go
type InventoryItem struct {
	domain.AggregateRootBase[guid.Guid]
	name string
}

type InventoryItemCreated struct {
	*cqrs.BaseEvent
	Id   guid.Guid
	Name string
}
```

Every aggregate needs a default constructor that wires its event handler. The
repository calls this constructor to get an empty aggregate before it replays
events into it, so keep it free of business logic.

```go
func DefaultInventoryItem() *InventoryItem {
	ii := InventoryItem{
		AggregateRootBase: domain.NewAggregate[guid.Guid](),
	}
	ii.SetInnerApply(ii.handleEvent)
	return &ii
}

func (ii *InventoryItem) handleEvent(e cqrs.Event) {
	switch evt := e.(type) {
	case InventoryItemCreated:
		ii.SetId(evt.Id)
		ii.SetVersion(evt.Ver)
		ii.name = evt.Name
	}
}
```

The handler mutates state and nothing else. Do not validate in it, because it
runs both for new events and for events replayed from storage.

Behaviour goes in methods that raise events. `cqrs.NewEvent` fills in the
message ID and version on the embedded `BaseEvent`, and `ApplyChange` runs the
handler and records the event as uncommitted.

```go
func NewInventoryItem(id guid.Guid, name string) *InventoryItem {
	i := DefaultInventoryItem()
	i.ApplyChange(cqrs.NewEvent[InventoryItemCreated](func(e *InventoryItemCreated) {
		e.Id = id
		e.Name = name
	}))
	return i
}
```

## Loading and saving

A repository pairs an event store with an aggregate's default constructor.
`GetById` replays the stored events into a fresh aggregate, and `Save` appends
the uncommitted ones.

```go
m := cqrs.NewMediator(false)
store := inmemory.NewInMemoryEventStore[guid.Guid](m)
repo := eventstore.NewRepository[*InventoryItem](store, DefaultInventoryItem)
```

`GetById` returns `eventstore.ErrAggregateNotFound` when the stream is empty.

## Expected versions

`Save` takes an expected version, and the store rejects the write if the stream
has moved on. Pass `-1` when you create an aggregate, which asserts that no
stream exists yet. Pass `aggregate.Version()` when you load and modify one,
which asserts that nothing has written to the stream since you read it.

```go
func (h Handlers) HandleRenameInventoryItem(cmd cqrs.Command) error {
	c := cmd.(RenameInventoryItem)

	item, err := h.repository.GetById(c.InventoryItemId)
	if err != nil {
		return err
	}

	expectedVersion := item.Version()
	item.Rename(c.NewName)

	return h.repository.Save(item, expectedVersion)
}
```

Saving a loaded aggregate with `-1` fails against any store that enforces the
check. The in-memory store treats `-1` as "do not check", so a mistake here
passes in unit tests and fails against Firestore.

## Dispatching commands and publishing events

The mediator routes commands to a single handler each, and events to any number
of processors. Register handlers before you dispatch anything.

```go
m := cqrs.NewMediator(false)
handlers := NewInventoryCommandHandler(repo)

cqrs.RegisterCommandHandler[CreateInventoryItem](m, handlers.HandleCreateInventoryItem)
cqrs.RegisterEventHandlers[InventoryItemCreated](m, readModel.HandleCreated)

m.Dispatch(NewCreateInventoryItem(guid.New(), "widget"), nil)
```

`Dispatch` returns an error straight away if no handler is registered for the
command type. Otherwise it queues the command for the mediator's own goroutine
and returns nil, so the handler has not run yet when it returns. To get the
handler's error back, pass a channel as the second argument and read from it.
`DispatchSync` runs the handler on the calling goroutine and returns its error
directly.

`Publish` calls each processor on its own goroutine, and `PublishSync` calls
them in turn on the calling goroutine. Both return an error when no processor
is registered for the event type.

Pass `true` to `NewMediator` to insert random delays before handling commands
and publishing events. Use it to shake out code that assumes a read model is
up to date the moment a command returns.

## Projections

A projection is a read model with an identifier and a version.
`BaseProjectionHandler` takes a load function, a save function, and a factory,
and handles the read-modify-write cycle.

```go
type InventoryItemReadModel struct {
	cqrs.BaseProjection
	name string
}

func (i Handler) HandleRenamed(e cqrs.Event) error {
	evt := e.(InventoryItemRenamed)

	return i.UpdateProjection(evt.Id, evt, func(p *InventoryItemReadModel, e cqrs.Event) {
		p.name = evt.NewName
	})
}
```

## Storage adapters

Both adapters need a type map, which tells the store how to turn a stored type
name back into a Go type. Register every event type an aggregate can raise.

```go
tm := store.NewTypeMap().
	Add(InventoryItemCreated{}).
	Add(InventoryItemRenamed{})

s, err := store.NewFirestoreEventStore(context.Background(), tm)
```

The MongoDB adapter takes a connection string instead of a context:

```go
s, err := store.NewMongoEventStore(store.ConnectionString("mongodb://localhost:27017"), tm)
```

Neither adapter publishes events. The in-memory store does, because it holds a
mediator, so a read model that updates in unit tests will not update against
Firestore or MongoDB. Publish from your command handlers if you need both.

The in-memory store also fails the save when the mediator has no processor
registered for an event it is publishing. Register a processor for every event
type your aggregates raise before you use it, even one that does nothing.

## Running the tests

The repository is a Go workspace, and `go.work` covers the core module and both
adapters. A pattern of `./...` matches only the module you are standing in, so
name the adapters as well:

```sh
go test ./... ./mongo/... -race
```

The Firestore tests run against the emulator, which needs a Java 21 or later
runtime on `PATH`. On macOS, `/usr/libexec/java_home -v 21` prints the path to
one.

```sh
gcloud emulators firestore start --host-port=127.0.0.1:8722 --project=iamkoch
FIRESTORE_EMULATOR_HOST=127.0.0.1:8722 go test ./firestore/... -race
```

## Continuous integration and releases

`.github/workflows/ci.yml` runs on every push to `main` and every pull
request. It builds, vets, and checks formatting across all three modules, runs
the core and MongoDB tests, then starts the Firestore emulator and runs the
Firestore tests. A second job builds each module with `GOWORK=off` and fails if
`go mod tidy` would change anything, which catches a module that imports a
package it does not require. A third runs `govulncheck` over all three.

Tagging is the release. Push a tag and `.github/workflows/release.yml` checks
that the tag names a real module, builds that module without the workspace,
creates the GitHub release, and asks the Go module proxy to fetch the version
so `go get` resolves it straight away.

```sh
git tag -a v0.1.2 -m 'v0.1.2'
git push origin v0.1.2
```

Tag the adapters with a `firestore/` or `mongo/` prefix. Tag the core module
first when the adapters need to require the new version, because the workspace
substitutes the code locally but Go still reads the go.mod of whatever version
they name.

## Known gaps

The MongoDB adapter has a Ginkgo suite with no specs in it, so it is compiled
but not exercised. The Firestore adapter hardcodes the project ID `iamkoch` in
`NewFirestoreEventStore`. `sample_domain` ships in the core module because the
adapter tests import it, which puts an example on the public API surface.
