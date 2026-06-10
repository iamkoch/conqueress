package mongo

import (
	"encoding/json"
	"fmt"

	"github.com/iamkoch/conqueress"
)

// TypeRegistry maps wire-stable event-type names to decoders. The store
// persists each event's name alongside its JSON body; on read, the registry
// turns the pair back into the concrete domain event the consumer's aggregate
// type switch expects.
//
// Names are explicit strings rather than Go type names, so renaming a Go type
// never breaks a persisted stream. The registry starts empty; consumers
// register their domain events at composition time:
//
//	registry := mongo.NewTypeRegistry()
//	mongo.Register[ItemCreated](registry, "item-created")
//	mongo.Register[ItemRenamed](registry, "item-renamed")
type TypeRegistry struct {
	decoders map[string]func([]byte) (conqueress.Event, error)
	names    map[string]string // Go type name -> wire name
}

// NewTypeRegistry returns an empty registry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		decoders: map[string]func([]byte) (conqueress.Event, error){},
		names:    map[string]string{},
	}
}

// Register wires a concrete event type to its wire name. T is the event's
// value type (aggregate type switches match values, not pointers); the
// decoder unmarshals into a pointer and dereferences. T must embed
// *conqueress.BaseEvent — encoding/json allocates the embedded pointer when
// its promoted fields appear in the body, which they always do (message_id is
// always present).
func Register[T any](r *TypeRegistry, name string) {
	var zero T
	goName := fmt.Sprintf("%T", zero)
	r.names[goName] = name
	r.decoders[name] = func(data []byte) (conqueress.Event, error) {
		t := new(T)
		if err := json.Unmarshal(data, t); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		evt, ok := any(*t).(conqueress.Event)
		if !ok {
			return nil, fmt.Errorf("registered type %s does not satisfy conqueress.Event", name)
		}
		return evt, nil
	}
}

// WireName returns the wire-stable name for a live event, or an error if the
// event's type was never registered.
func (r *TypeRegistry) WireName(e conqueress.Event) (string, error) {
	goName := fmt.Sprintf("%T", e)
	name, ok := r.names[goName]
	if !ok {
		return "", fmt.Errorf("event type %s is not registered in the TypeRegistry", goName)
	}
	return name, nil
}

// Encode renders an event to its wire name and JSON body.
func (r *TypeRegistry) Encode(e conqueress.Event) (name string, body []byte, err error) {
	name, err = r.WireName(e)
	if err != nil {
		return "", nil, err
	}
	body, err = json.Marshal(e)
	if err != nil {
		return "", nil, fmt.Errorf("encode %s: %w", name, err)
	}
	return name, body, nil
}

// Decode turns a persisted (name, body) pair back into the concrete event.
func (r *TypeRegistry) Decode(name string, body []byte) (conqueress.Event, error) {
	decode, ok := r.decoders[name]
	if !ok {
		return nil, fmt.Errorf("no decoder registered for event type %q", name)
	}
	return decode(body)
}
