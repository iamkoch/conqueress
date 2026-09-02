package store

import (
	"fmt"
	"os"
	"sync"
	"testing"

	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/example"
	"github.com/iamkoch/conqueress/guid"
	"github.com/iamkoch/ensure"
	"github.com/stretchr/testify/require"
)

// The store writes events and aggregates in one transaction, which MongoDB
// only supports on a replica set. README.md has the docker command that starts
// one. Point MONGO_CONNECTION_STRING at a different deployment to use that
// instead.
const defaultConnectionString = "mongodb://127.0.0.1:27017/?replicaSet=rs0"

func connectionString() ConnectionString {
	if cs := os.Getenv("MONGO_CONNECTION_STRING"); cs != "" {
		return ConnectionString(cs)
	}
	return defaultConnectionString
}

// newStore gives each test its own database, so tests cannot see each other's
// aggregates and can run in any order.
func newStore(t *testing.T) eventstore.IEventStore {
	t.Helper()

	tm := NewTypeMap().
		Add(example.InventoryItemCreated{}).
		Add(example.InventoryItemRenamed{})

	es, err := NewMongoEventStore(connectionString(), fmt.Sprintf("test_%s", guid.New()), tm)
	require.NoError(t, err, "connecting to mongo")

	return es
}

func newRepo(t *testing.T) eventstore.Repository[*example.InventoryItem] {
	t.Helper()
	return eventstore.NewRepository[*example.InventoryItem](newStore(t), example.DefaultInventoryItem)
}

func TestSaveAndLoad(t *testing.T) {
	var (
		repo    = newRepo(t)
		itemId  = guid.New()
		loaded  *example.InventoryItem
		loadErr error
		saveErr error
	)

	ensure.That("an aggregate survives a save and a load", func(s *ensure.Scenario) {
		s.Given("a new inventory item", func() {
			saveErr = repo.Save(example.NewInventoryItem(itemId, "original"), -1)
		})

		s.Then("saving it should succeed", func() {
			require.NoError(t, saveErr)
		})

		s.When("I load it back", func() {
			loaded, loadErr = repo.GetById(itemId)
		})

		s.Then("I should get no error", func() {
			require.NoError(t, loadErr)
		})

		s.And("it should carry the name it was created with", func() {
			require.Equal(t, "original", loaded.Name())
		})

		s.And("it should be at version 0", func() {
			require.Equal(t, 0, loaded.Version())
		})
	}, t)
}

func TestRenameAdvancesTheVersion(t *testing.T) {
	var (
		repo     = newRepo(t)
		itemId   = guid.New()
		reloaded *example.InventoryItem
		err      error
	)

	ensure.That("a second event advances the aggregate's version", func(s *ensure.Scenario) {
		s.Background("Given a saved inventory item", func() {
			require.NoError(t, repo.Save(example.NewInventoryItem(itemId, "original"), -1))
		})

		s.When("I load it, rename it, and save it at its current version", func() {
			item, loadErr := repo.GetById(itemId)
			require.NoError(t, loadErr)

			item.Rename("renamed")
			err = repo.Save(item, item.Version())
		})

		s.Then("the save should succeed", func() {
			require.NoError(t, err)
		})

		s.When("I load it again", func() {
			reloaded, err = repo.GetById(itemId)
		})

		s.Then("it should carry the new name", func() {
			require.NoError(t, err)
			require.Equal(t, "renamed", reloaded.Name())
		})

		s.And("it should be at version 1", func() {
			require.Equal(t, 1, reloaded.Version())
		})
	}, t)
}

func TestSavingAtAStaleVersionIsRejected(t *testing.T) {
	var (
		repo    = newRepo(t)
		itemId  = guid.New()
		saveErr error
	)

	ensure.That("saving at a version another writer has moved past is rejected", func(s *ensure.Scenario) {
		s.Background("Given an inventory item that has been renamed once", func() {
			require.NoError(t, repo.Save(example.NewInventoryItem(itemId, "original"), -1))

			item, err := repo.GetById(itemId)
			require.NoError(t, err)

			item.Rename("renamed")
			require.NoError(t, repo.Save(item, item.Version()))
		})

		s.When("I save it again at the version it had before that rename", func() {
			item, err := repo.GetById(itemId)
			require.NoError(t, err)

			item.Rename("renamed by a stale writer")
			saveErr = repo.Save(item, 0)
		})

		s.Then("I should get a concurrency exception", func() {
			require.Error(t, saveErr)
		})
	}, t)
}

// TestConcurrentWritersAtSameVersion is the MongoDB counterpart of the
// Firestore test of the same name. The version check has to be part of the
// transaction, so that of several writers holding the same expected version
// exactly one commits.
func TestConcurrentWritersAtSameVersion(t *testing.T) {
	const writers = 8

	var (
		repo            = newRepo(t)
		itemId          = guid.New()
		loaded          = make([]*example.InventoryItem, writers)
		results         = make([]error, writers)
		expectedVersion int
		succeeded       int
	)

	ensure.That("only one of several writers at the same version commits", func(s *ensure.Scenario) {
		s.Background("Given a saved inventory item", func() {
			require.NoError(t, repo.Save(example.NewInventoryItem(itemId, "original"), -1))
		})

		s.And("every writer has loaded it, so they all hold the same version", func() {
			for i := range loaded {
				item, err := repo.GetById(itemId)
				require.NoError(t, err)
				loaded[i] = item
			}
			expectedVersion = loaded[0].Version()
		})

		s.When("they all rename it and save at once", func() {
			var (
				wg    sync.WaitGroup
				mu    sync.Mutex
				start = make(chan struct{})
			)

			for i := 0; i < writers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()

					item := loaded[i]
					item.Rename(fmt.Sprintf("renamed by %d", i))

					<-start
					err := repo.Save(item, expectedVersion)

					mu.Lock()
					defer mu.Unlock()
					results[i] = err
				}(i)
			}

			close(start)
			wg.Wait()

			for _, err := range results {
				if err == nil {
					succeeded++
				}
			}
		})

		s.Then("exactly one of them should have committed", func() {
			require.Equal(t, 1, succeeded,
				"exactly one writer may commit at version %d, but %d did", expectedVersion, succeeded)
		})

		s.And("the stream should have advanced by exactly one event", func() {
			reloaded, err := repo.GetById(itemId)
			require.NoError(t, err)
			require.Equal(t, expectedVersion+1, reloaded.Version())
		})
	}, t)
}

func TestEventsComeBackInOrder(t *testing.T) {
	var (
		store  = newStore(t)
		itemId = guid.New()
		events []cqrs.Event
	)

	ensure.That("an aggregate's events read back in the order they were saved", func(s *ensure.Scenario) {
		s.Background("Given an item that has been created and renamed", func() {
			repo := eventstore.NewRepository[*example.InventoryItem](store, example.DefaultInventoryItem)
			require.NoError(t, repo.Save(example.NewInventoryItem(itemId, "original"), -1))

			item, err := repo.GetById(itemId)
			require.NoError(t, err)

			item.Rename("renamed")
			require.NoError(t, repo.Save(item, item.Version()))
		})

		s.When("I read the aggregate's events", func() {
			events = store.GetEventsForAggregate(itemId)
		})

		s.Then("I should get both of them", func() {
			require.Len(t, events, 2)
		})

		s.And("they should be in the order they were saved", func() {
			require.IsType(t, example.InventoryItemCreated{}, events[0])
			require.IsType(t, example.InventoryItemRenamed{}, events[1])
		})
	}, t)
}
