package mongo

import (
	"testing"
	"time"

	"github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	"github.com/stretchr/testify/assert"
)

// itemID is a composite aggregate id, proving the store is generic over any
// Stringer-able ID, not just guid.Guid.
type itemID struct {
	Tenant string
	Item   string
}

func (i itemID) String() string { return i.Tenant + "/" + i.Item }

type itemCreated struct {
	*conqueress.BaseEvent
	Name string
}

type itemRenamed struct {
	*conqueress.BaseEvent
	NewName string
}

// Compile-time conformance: a Store over a composite ID satisfies the
// context-aware port.
var _ eventstore.Store[itemID] = (*Store[itemID])(nil)

func newTestRegistry() *TypeRegistry {
	r := NewTypeRegistry()
	Register[itemCreated](r, "item-created")
	Register[itemRenamed](r, "item-renamed")
	return r
}

func TestRegistry_RoundTripsWithMetadata(t *testing.T) {
	r := newTestRegistry()
	correlation := guid.New()
	causation := guid.New()
	occurred := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	original := conqueress.NewEvent[itemCreated](func(e *itemCreated) { e.Name = "widget" })
	original.WithMetadata(correlation, causation, occurred)
	original.WithVersion(7)

	name, body, err := r.Encode(original)
	assert.NoError(t, err)
	assert.Equal(t, "item-created", name)

	decoded, err := r.Decode(name, body)
	assert.NoError(t, err)

	created, ok := decoded.(itemCreated)
	assert.True(t, ok, "decoded event is the concrete value type")
	assert.Equal(t, original.MsgId(), created.MsgId())
	assert.Equal(t, 7, created.Version())
	assert.Equal(t, correlation, created.CorrelationId())
	assert.Equal(t, causation, created.CausationId())
	assert.Equal(t, occurred, created.OccurredAt())
	assert.Equal(t, "widget", created.Name)
}

func TestRegistry_UnregisteredTypeFailsLoudly(t *testing.T) {
	r := newTestRegistry()

	type rogue struct{ *conqueress.BaseEvent }
	_, _, err := r.Encode(conqueress.NewEvent[rogue]())
	assert.ErrorContains(t, err, "not registered")

	_, err = r.Decode("never-registered", []byte(`{}`))
	assert.ErrorContains(t, err, "no decoder registered")
}

func TestGuidOrEmpty_MapsEmptyGuidToAbsent(t *testing.T) {
	assert.Equal(t, "", guidOrEmpty(guid.Empty))
	g := guid.New()
	assert.Equal(t, g.String(), guidOrEmpty(g))
}
