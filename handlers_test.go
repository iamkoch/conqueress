package conqueress

import (
	"context"
	"errors"
	"testing"

	"github.com/iamkoch/conqueress/guid"
	"github.com/stretchr/testify/assert"
)

// MakeCreateThingHandler is a representative factory composition:
// closes over a fake repository and returns a typed HandleCommand[T].
func makeCreateThingHandler(saved *[]string) HandleCommand[createThing] {
	return func(_ context.Context, cmd createThing, _, _ guid.Guid) error {
		if cmd.Name == "" {
			return errors.New("name required")
		}
		*saved = append(*saved, cmd.Name)
		return nil
	}
}

type createThing struct {
	BaseCommand
	Name string
}

func TestHandleCommand_TypedHandlerComposition(t *testing.T) {
	var saved []string
	handle := makeCreateThingHandler(&saved)

	err := handle(context.Background(), createThing{Name: "widget"}, guid.New(), guid.New())

	assert.NoError(t, err)
	assert.Equal(t, []string{"widget"}, saved)
}

func TestHandleCommand_PropagatesFailureFromHandler(t *testing.T) {
	var saved []string
	handle := makeCreateThingHandler(&saved)

	err := handle(context.Background(), createThing{Name: ""}, guid.New(), guid.New())

	assert.Error(t, err)
	assert.Empty(t, saved)
}

// fakeIntegrationEvent satisfies IntegrationEvent for the publisher test.
type fakeIntegrationEvent struct {
	*BaseEvent
}

func (fakeIntegrationEvent) EventType() string { return "test.fake" }

type fakePublisher struct {
	published []IntegrationEvent
}

func (p *fakePublisher) Publish(_ context.Context, evt IntegrationEvent, _, _ guid.Guid) error {
	p.published = append(p.published, evt)
	return nil
}

func TestIntegrationEventPublisher_AcceptsAnyIntegrationEvent(t *testing.T) {
	var pub IntegrationEventPublisher = &fakePublisher{}
	evt := NewEvent[fakeIntegrationEvent]()

	err := pub.Publish(context.Background(), evt, guid.New(), guid.New())

	assert.NoError(t, err)
	assert.Equal(t, 1, len(pub.(*fakePublisher).published))
	assert.Equal(t, "test.fake", pub.(*fakePublisher).published[0].(IntegrationEvent).EventType())
}
